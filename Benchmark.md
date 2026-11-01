
## Benchmark Against  s3:// with rust-fs (without fuse overhead)

### Summary

DuckDB over an fs-relay mount, against the same files read from rustfs with `s3://`.

Each query starts cold, on both paths, so the timer is the storage path and the second query cannot reuse the first.

- DuckDB is quit and started again. `SET enable_external_file_cache = false` is set again in the new session. That cache keeps parquet footers and pages inside the DuckDB process. With it left on, the second scan would time DuckDB's own memory. With it off, every query reads the files through S3 or through the mount.
- The kernel page cache is dropped on the machine that holds the files, and on the client before a mount query:

```bash
sync; echo 3 > /proc/sys/vm/drop_caches
```

`sync` writes dirty pages out. Writing `3` drops the page cache, dentries, and inodes, so the next read comes off disk and then across the LAN. This needs root.

- rustfs is restarted between its two queries, so the daemon's buffers are empty too.
- Between the two mount queries the filesystem is unmounted, `fs-relay` is restarted, and `fs-relay-bind` is mounted again. That clears the kernel's name, attribute, and page caches for the mount, and anything the server still held.

| Query | rustfs S3 | fs-relay |
|---|---:|---:|
| by `pickup_year` | 21.17 s | 4.74 s (4.5x faster) |
| by `payment_type` | 22.76 s | 0.50 s (45x faster) |

### The Data

the setup, two machines on same lan (can be done with one)

on the storage machine

download [nyc_taxi_2009-2010.parquet](https://datasets-documentation.s3.eu-west-3.amazonaws.com/nyc-taxi/clickhouse-academy/nyc_taxi_2009-2010.parquet)

then partition it using duckdb

start rustfs (as systemd if you will)

```bash
rustfs --access-key abc --secret-key abc /path/to/rustfs-data/
```

```sql
COPY (
  SELECT
    *, YEAR(pickup_datetime) AS pickup_year, MONTH(pickup_datetime) AS pickup_month
  FROM "./nyc_taxi_2009-2010.parquet"
) TO './storage/trips' (
    FORMAT PARQUET,
    PARTITION BY (pickup_year, pickup_month),
    PARQUET_VERSION 'V2',
    COMPRESSION 'LZ4'
);
```

upload it to rust-fs s3 with

```bash
AWS_ACCESS_KEY_ID=abc AWS_SECRET_ACCESS_KEY=abc aws --endpoint-url http://127.0.0.1:9000 s3 sync ./storage/trips s3://trips
```

run the fs-relay server

```bash
fs-relay -data $PWD/storage -listen 0.0.0.0:1300 -token trips-token
```

### DuckDB + RustFS in S3 (no fuse)

on the other machine run the SQL query, run duckdb and connect to the rust-fs s3


```sql
SET enable_external_file_cache = false;

LOAD httpfs;
SET s3_endpoint='storage-ip:9000';
SET s3_region='us-east-1';
SET s3_use_ssl=false;
SET s3_access_key_id='abc';
SET s3_secret_access_key='abc';
SET s3_url_style='path';    -- This is important
```

and here is the query

```
D .timer on
D SELECT payment_type, avg(total_amount), sum(total_amount)
    FROM read_parquet('s3://trips/*/*/*.parquet', hive_partitioning = true)
    GROUP BY payment_type;
100% ▕██████████████████████████████████████▏ (00:00:22.72 elapsed)     
┌──────────────┬────────────────────┬────────────────────┐
│ payment_type │ avg(total_amount)  │ sum(total_amount)  │
│    int64     │       double       │       double       │
├──────────────┼────────────────────┼────────────────────┤
│            1 │ 14.191653788135877 │ 1458493176.2580395 │
│            2 │  9.600059365112047 │ 2163631417.9236746 │
│            4 │ 12.295325249019207 │ 1714546.2199999813 │
│         NULL │ 11.354581776206603 │  786929.2899999986 │
│            3 │ 10.539190662135784 │  7244133.780000355 │
└──────────────┴────────────────────┴────────────────────┘
Run Time (s): real 22.763 user 4.255886 sys 0.767897
```

between the queries restart duckdb and make sure `SET enable_external_file_cache = false;`
and restart rustfs and `sync; echo 3 > /proc/sys/vm/drop_caches`

```
D SELECT
    pickup_year, count(*), sum(total_amount), avg(total_amount)
  FROM
    read_parquet('s3://trips/*/*/*.parquet', hive_partitioning = true)
  GROUP BY pickup_year;
100% ▕██████████████████████████████████████▏ (00:00:21.12 elapsed)     
┌─────────────┬──────────────┬────────────────────┬────────────────────┐
│ pickup_year │ count_star() │ sum(total_amount)  │ avg(total_amount)  │
│    int64    │    int64     │       double       │       double       │
├─────────────┼──────────────┼────────────────────┼────────────────────┤
│        2011 │        77266 │  885290.2399999942 │ 11.457694716951753 │
│        2010 │    161955906 │ 1819708916.0599458 │ 11.235829313072077 │
│        2009 │    167011003 │ 1811275997.1899757 │ 10.845249502453294 │
└─────────────┴──────────────┴────────────────────┴────────────────────┘
Run Time (s): real 21.169 user 3.503777 sys 0.553657
```

### DuckDB + fs-relay

mount the fs-relay to the local machine

```bash
mkdir -p ./mnt/trips
fs-relay-bind -ns trips -url http://storage-ip:1300 -token trips-token ./mnt/trips
```

run the query

```
D SET enable_external_file_cache = false;
D .timer on
D SELECT pickup_year, count(*), sum(total_amount), avg(total_amount)
      FROM read_parquet('mnt/trips/*/*/*.parquet', hive_partitioning = true)
      GROUP BY pickup_year;
100% ▕██████████████████████████████████████▏ (00:00:04.71 elapsed)     
┌─────────────┬──────────────┬────────────────────┬────────────────────┐
│ pickup_year │ count_star() │ sum(total_amount)  │ avg(total_amount)  │
│    int64    │    int64     │       double       │       double       │
├─────────────┼──────────────┼────────────────────┼────────────────────┤
│        2011 │        77266 │  885290.2399999942 │ 11.457694716951753 │
│        2010 │    161955906 │ 1819708916.0599463 │  11.23582931307208 │
│        2009 │    167011003 │ 1811275997.1899753 │ 10.845249502453292 │
└─────────────┴──────────────┴────────────────────┴────────────────────┘
Run Time (s): real 4.740 user 3.210861 sys 1.002303
```

between the queries restart duckdb and make sure `SET enable_external_file_cache = false;`
and unmount the fs-relay, restart its server, then restart its client and of course run `sync; echo 3 > /proc/sys/vm/drop_caches`


```
D SET enable_external_file_cache = false;
D .timer on
D SELECT payment_type, avg(total_amount), sum(total_amount)
  FROM read_parquet('mnt/trips/*/*/*.parquet', hive_partitioning = true)
  GROUP BY payment_type;
┌──────────────┬────────────────────┬────────────────────┐
│ payment_type │ avg(total_amount)  │ sum(total_amount)  │
│    int64     │       double       │       double       │
├──────────────┼────────────────────┼────────────────────┤
│            2 │  9.600059365113601 │  2163631417.924025 │
│            4 │ 12.295325249019204 │ 1714546.2199999809 │
│            3 │ 10.539190662135779 │  7244133.780000351 │
│         NULL │ 11.354581776206606 │  786929.2899999989 │
│            1 │ 14.191653788135893 │ 1458493176.2580411 │
└──────────────┴────────────────────┴────────────────────┘
Run Time (s): real 0.502 user 3.670408 sys 0.155117

```

