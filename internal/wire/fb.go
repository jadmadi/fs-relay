package wire

import (
	"encoding/binary"
	"fmt"
	"io"

	flatbuffers "github.com/google/flatbuffers/go"
)

const fbID = "FSR1"

type fbMsg struct {
	ID      uint64
	Method  string
	Params  *Params
	Result  *Result
	Err     *Error
	Fault   string
	Pending string
	Evict   *Change
	Release string
	Client  string
}

func encodeFB(msg fbMsg) ([]byte, error) {
	b := flatbuffers.NewBuilder(len(fileBytes(msg)) + 256)
	params := putParams(b, msg.Params)
	result := putResult(b, msg.Result)
	failure := putFailure(b, msg.Err)
	evict := putChange(b, msg.Evict)
	method := putStr(b, msg.Method)
	fault := putStr(b, msg.Fault)
	pending := putStr(b, msg.Pending)
	release := putStr(b, msg.Release)
	client := putStr(b, msg.Client)
	FbEnvelopeStart(b)
	addEnvelope(b, msg, method, params, result, failure, evict, fault, pending, release, client)
	b.FinishWithFileIdentifier(FbEnvelopeEnd(b), []byte(fbID))
	return b.FinishedBytes(), nil
}

func fileBytes(msg fbMsg) []byte {
	if msg.Params != nil {
		return msg.Params.Data
	}
	if msg.Result != nil {
		return msg.Result.Data
	}
	return nil
}

func addEnvelope(b *flatbuffers.Builder, msg fbMsg, method, params, result, failure, evict, fault, pending, release, client flatbuffers.UOffsetT) {
	if msg.ID != 0 {
		FbEnvelopeAddId(b, msg.ID)
	}
	addOffset(b, method, FbEnvelopeAddMethod)
	addOffset(b, params, FbEnvelopeAddParams)
	addOffset(b, result, FbEnvelopeAddResult)
	addOffset(b, failure, FbEnvelopeAddFailure)
	addOffset(b, evict, FbEnvelopeAddEvict)
	addOffset(b, fault, FbEnvelopeAddFault)
	addOffset(b, pending, FbEnvelopeAddPending)
	addOffset(b, release, FbEnvelopeAddRelease)
	addOffset(b, client, FbEnvelopeAddClient)
}

func addOffset(b *flatbuffers.Builder, off flatbuffers.UOffsetT, add func(*flatbuffers.Builder, flatbuffers.UOffsetT)) {
	if off != 0 {
		add(b, off)
	}
}

func putStr(b *flatbuffers.Builder, s string) flatbuffers.UOffsetT {
	if s == "" {
		return 0
	}
	return b.CreateString(s)
}

func putBytes(b *flatbuffers.Builder, p []byte) flatbuffers.UOffsetT {
	if len(p) == 0 {
		return 0
	}
	return b.CreateByteVector(p)
}

func putParams(b *flatbuffers.Builder, p *Params) flatbuffers.UOffsetT {
	if p == nil {
		return 0
	}
	ns := putStr(b, p.Namespace)
	name := putStr(b, p.Name)
	newName := putStr(b, p.NewName)
	target := putStr(b, p.Target)
	client := putStr(b, p.Client)
	data := putBytes(b, p.Data)
	inodes := putInodes(b, p.Inodes)
	FbParamsStart(b)
	addOffset(b, ns, FbParamsAddNamespace)
	FbParamsAddInode(b, p.Inode)
	FbParamsAddNewInode(b, p.NewInode)
	addOffset(b, name, FbParamsAddName)
	addOffset(b, newName, FbParamsAddNewName)
	FbParamsAddFh(b, p.Fh)
	FbParamsAddMode(b, p.Mode)
	FbParamsAddFlags(b, p.Flags)
	FbParamsAddValid(b, p.Valid)
	FbParamsAddMask(b, p.Mask)
	FbParamsAddOffset(b, p.Offset)
	FbParamsAddSize(b, p.Size)
	FbParamsAddAtime(b, p.Atime)
	FbParamsAddMtime(b, p.Mtime)
	FbParamsAddUid(b, p.UID)
	FbParamsAddGid(b, p.GID)
	addOffset(b, target, FbParamsAddTarget)
	addOffset(b, data, FbParamsAddData)
	addOffset(b, inodes, FbParamsAddInodes)
	addOffset(b, client, FbParamsAddClient)
	FbParamsAddFhOut(b, p.FhOut)
	FbParamsAddOffsetOut(b, p.OffsetOut)
	return FbParamsEnd(b)
}

func putInodes(b *flatbuffers.Builder, v []uint64) flatbuffers.UOffsetT {
	if len(v) == 0 {
		return 0
	}
	FbParamsStartInodesVector(b, len(v))
	for i := len(v) - 1; i >= 0; i-- {
		b.PrependUint64(v[i])
	}
	return b.EndVector(len(v))
}

func putResult(b *flatbuffers.Builder, res *Result) flatbuffers.UOffsetT {
	if res == nil {
		return 0
	}
	attr := putAttr(b, res.Attr)
	entries := putDirents(b, res.Entries)
	data := putBytes(b, res.Data)
	target := putStr(b, res.Target)
	stat := putStat(b, res.Stat)
	blobs := putBlobs(b, res.Blobs)
	listings := putListings(b, res.Listings)
	FbResultStart(b)
	addOffset(b, attr, FbResultAddAttr)
	addOffset(b, entries, FbResultAddEntries)
	FbResultAddFh(b, res.Fh)
	addOffset(b, data, FbResultAddData)
	addOffset(b, target, FbResultAddTarget)
	addOffset(b, stat, FbResultAddStatfs)
	FbResultAddWritten(b, res.Written)
	addOffset(b, blobs, FbResultAddBlobs)
	addOffset(b, listings, FbResultAddListings)
	return FbResultEnd(b)
}

func putAttr(b *flatbuffers.Builder, a *Attr) flatbuffers.UOffsetT {
	if a == nil {
		return 0
	}
	FbAttrStart(b)
	FbAttrAddIno(b, a.Ino)
	FbAttrAddMode(b, a.Mode)
	FbAttrAddSize(b, a.Size)
	FbAttrAddNlink(b, a.Nlink)
	FbAttrAddUid(b, a.Uid)
	FbAttrAddGid(b, a.Gid)
	FbAttrAddAtime(b, a.Atime)
	FbAttrAddMtime(b, a.Mtime)
	FbAttrAddCtime(b, a.Ctime)
	return FbAttrEnd(b)
}

func putDirents(b *flatbuffers.Builder, entries []Dirent) flatbuffers.UOffsetT {
	if len(entries) == 0 {
		return 0
	}
	offs := make([]flatbuffers.UOffsetT, len(entries))
	for i := range entries {
		offs[i] = putDirent(b, entries[i])
	}
	FbResultStartEntriesVector(b, len(offs))
	for i := len(offs) - 1; i >= 0; i-- {
		b.PrependUOffsetT(offs[i])
	}
	return b.EndVector(len(offs))
}

func putDirent(b *flatbuffers.Builder, d Dirent) flatbuffers.UOffsetT {
	name := putStr(b, d.Name)
	FbDirentStart(b)
	addOffset(b, name, FbDirentAddName)
	FbDirentAddIno(b, d.Ino)
	FbDirentAddMode(b, d.Mode)
	FbDirentAddSize(b, d.Size)
	FbDirentAddNlink(b, d.Nlink)
	FbDirentAddUid(b, d.Uid)
	FbDirentAddGid(b, d.Gid)
	FbDirentAddAtime(b, d.Atime)
	FbDirentAddMtime(b, d.Mtime)
	FbDirentAddCtime(b, d.Ctime)
	return FbDirentEnd(b)
}

func putStat(b *flatbuffers.Builder, s *FSStat) flatbuffers.UOffsetT {
	if s == nil {
		return 0
	}
	FbFSStatStart(b)
	FbFSStatAddBlocks(b, s.Blocks)
	FbFSStatAddBfree(b, s.Bfree)
	FbFSStatAddBavail(b, s.Bavail)
	FbFSStatAddFiles(b, s.Files)
	FbFSStatAddFfree(b, s.Ffree)
	FbFSStatAddBsize(b, s.Bsize)
	FbFSStatAddNamelen(b, s.NameLen)
	FbFSStatAddFrsize(b, s.Frsize)
	return FbFSStatEnd(b)
}

func putBlobs(b *flatbuffers.Builder, blobs []Blob) flatbuffers.UOffsetT {
	if len(blobs) == 0 {
		return 0
	}
	offs := make([]flatbuffers.UOffsetT, len(blobs))
	for i := range blobs {
		offs[i] = putBlob(b, blobs[i])
	}
	FbResultStartBlobsVector(b, len(offs))
	for i := len(offs) - 1; i >= 0; i-- {
		b.PrependUOffsetT(offs[i])
	}
	return b.EndVector(len(offs))
}

func putListings(b *flatbuffers.Builder, listings []Listing) flatbuffers.UOffsetT {
	if len(listings) == 0 {
		return 0
	}
	offs := make([]flatbuffers.UOffsetT, len(listings))
	for i := range listings {
		offs[i] = putListing(b, listings[i])
	}
	FbResultStartListingsVector(b, len(offs))
	for i := len(offs) - 1; i >= 0; i-- {
		b.PrependUOffsetT(offs[i])
	}
	return b.EndVector(len(offs))
}

func putListing(b *flatbuffers.Builder, listing Listing) flatbuffers.UOffsetT {
	ents := putListingEntries(b, listing.Entries)
	FbListingStart(b)
	FbListingAddIno(b, listing.Inode)
	addOffset(b, ents, FbListingAddEntries)
	return FbListingEnd(b)
}

func putListingEntries(b *flatbuffers.Builder, entries []Dirent) flatbuffers.UOffsetT {
	if len(entries) == 0 {
		return 0
	}
	offs := make([]flatbuffers.UOffsetT, len(entries))
	for i := range entries {
		offs[i] = putDirent(b, entries[i])
	}
	FbListingStartEntriesVector(b, len(offs))
	for i := len(offs) - 1; i >= 0; i-- {
		b.PrependUOffsetT(offs[i])
	}
	return b.EndVector(len(offs))
}

func putBlob(b *flatbuffers.Builder, blob Blob) flatbuffers.UOffsetT {
	data := putBytes(b, blob.Data)
	FbBlobStart(b)
	FbBlobAddIno(b, blob.Inode)
	addOffset(b, data, FbBlobAddData)
	return FbBlobEnd(b)
}

func putFailure(b *flatbuffers.Builder, e *Error) flatbuffers.UOffsetT {
	if e == nil {
		return 0
	}
	message := putStr(b, e.Message)
	FbFailureStart(b)
	FbFailureAddCode(b, e.Code)
	addOffset(b, message, FbFailureAddMessage)
	return FbFailureEnd(b)
}

func putChange(b *flatbuffers.Builder, c *Change) flatbuffers.UOffsetT {
	if c == nil {
		return 0
	}
	name := putStr(b, c.Name)
	FbChangeStart(b)
	FbChangeAddParent(b, c.Parent)
	addOffset(b, name, FbChangeAddName)
	return FbChangeEnd(b)
}

func fbRoot(buf []byte) (*FbEnvelope, error) {
	if !flatbuffers.BufferHasIdentifier(buf, fbID) {
		return nil, fmt.Errorf("flatbuffer: %w", ErrParams)
	}
	return GetRootAsFbEnvelope(buf, 0), nil
}

func decodeFBRequest(buf []byte) (Request, error) {
	env, err := fbRoot(buf)
	if err != nil {
		return Request{}, err
	}
	req := Request{ID: env.Id(), Method: string(env.Method())}
	if p := env.Params(new(FbParams)); p != nil {
		req.Params = paramsFrom(p)
	}
	return req, nil
}

func paramsFrom(p *FbParams) Params {
	return Params{
		Namespace: string(p.Namespace()),
		Inode:     p.Inode(),
		NewInode:  p.NewInode(),
		Name:      string(p.Name()),
		NewName:   string(p.NewName()),
		Fh:        p.Fh(),
		Mode:      p.Mode(),
		Flags:     p.Flags(),
		Valid:     p.Valid(),
		Mask:      p.Mask(),
		Offset:    p.Offset(),
		Size:      p.Size(),
		Atime:     p.Atime(),
		Mtime:     p.Mtime(),
		UID:       p.Uid(),
		GID:       p.Gid(),
		Target:    string(p.Target()),
		Data:      p.DataBytes(),
		Inodes:    inodesFrom(p),
		Client:    string(p.Client()),
		FhOut:     p.FhOut(),
		OffsetOut: p.OffsetOut(),
	}
}

func inodesFrom(p *FbParams) []uint64 {
	n := p.InodesLength()
	if n == 0 {
		return nil
	}
	out := make([]uint64, n)
	for i := 0; i < n; i++ {
		out[i] = p.Inodes(i)
	}
	return out
}

func decodeFBReply(buf []byte) (Reply, error) {
	env, err := fbRoot(buf)
	if err != nil {
		return Reply{}, err
	}
	rp := Reply{ID: env.Id(), Client: string(env.Client()), Fault: string(env.Fault()), Pending: string(env.Pending()), Release: string(env.Release())}
	if res := env.Result(new(FbResult)); res != nil {
		got := resultFrom(res)
		rp.Result = &got
	}
	if e := env.Failure(new(FbFailure)); e != nil {
		rp.Err = &Error{Code: e.Code(), Message: string(e.Message())}
	}
	if c := env.Evict(new(FbChange)); c != nil {
		rp.Evict = &Change{Parent: c.Parent(), Name: string(c.Name())}
	}
	return rp, nil
}

func resultFrom(res *FbResult) Result {
	out := Result{
		Fh:       res.Fh(),
		Data:     res.DataBytes(),
		Target:   string(res.Target()),
		Written:  res.Written(),
		Entries:  direntsFrom(res),
		Blobs:    blobsFrom(res),
		Listings: listingsFrom(res),
	}
	if a := res.Attr(new(FbAttr)); a != nil {
		attr := attrFrom(a)
		out.Attr = &attr
	}
	if s := res.Statfs(new(FbFSStat)); s != nil {
		stat := statFrom(s)
		out.Stat = &stat
	}
	return out
}

func attrFrom(a *FbAttr) Attr {
	return Attr{Ino: a.Ino(), Mode: a.Mode(), Size: a.Size(), Nlink: a.Nlink(), Uid: a.Uid(), Gid: a.Gid(), Atime: a.Atime(), Mtime: a.Mtime(), Ctime: a.Ctime()}
}

func direntsFrom(res *FbResult) []Dirent {
	n := res.EntriesLength()
	if n == 0 {
		return nil
	}
	out := make([]Dirent, n)
	for i := 0; i < n; i++ {
		var d FbDirent
		if !res.Entries(&d, i) {
			continue
		}
		out[i] = Dirent{Name: string(d.Name()), Attr: attrFromDir(&d)}
	}
	return out
}

func attrFromDir(d *FbDirent) Attr {
	return Attr{Ino: d.Ino(), Mode: d.Mode(), Size: d.Size(), Nlink: d.Nlink(), Uid: d.Uid(), Gid: d.Gid(), Atime: d.Atime(), Mtime: d.Mtime(), Ctime: d.Ctime()}
}

func listingsFrom(res *FbResult) []Listing {
	n := res.ListingsLength()
	if n == 0 {
		return nil
	}
	out := make([]Listing, n)
	for i := 0; i < n; i++ {
		var listing FbListing
		if !res.Listings(&listing, i) {
			continue
		}
		out[i] = Listing{Inode: listing.Ino(), Entries: listingEntries(&listing)}
	}
	return out
}

func listingEntries(listing *FbListing) []Dirent {
	n := listing.EntriesLength()
	if n == 0 {
		return nil
	}
	out := make([]Dirent, n)
	for i := 0; i < n; i++ {
		var d FbDirent
		if !listing.Entries(&d, i) {
			continue
		}
		out[i] = Dirent{Name: string(d.Name()), Attr: attrFromDir(&d)}
	}
	return out
}

func blobsFrom(res *FbResult) []Blob {
	n := res.BlobsLength()
	if n == 0 {
		return nil
	}
	out := make([]Blob, n)
	for i := 0; i < n; i++ {
		var blob FbBlob
		if !res.Blobs(&blob, i) {
			continue
		}
		out[i] = Blob{Inode: blob.Ino(), Data: blob.DataBytes()}
	}
	return out
}

func statFrom(s *FbFSStat) FSStat {
	return FSStat{Blocks: s.Blocks(), Bfree: s.Bfree(), Bavail: s.Bavail(), Files: s.Files(), Ffree: s.Ffree(), Bsize: s.Bsize(), NameLen: s.Namelen(), Frsize: s.Frsize()}
}

func decodeFBPub(buf []byte) (Pub, error) {
	env, err := fbRoot(buf)
	if err != nil {
		return Pub{}, err
	}
	p := Pub{Client: string(env.Client()), Release: string(env.Release())}
	if c := env.Evict(new(FbChange)); c != nil {
		p.Evict = &Change{Parent: c.Parent(), Name: string(c.Name())}
	}
	return p, nil
}

// WriteFrame prefixes one FlatBuffers join message with its length.
func WriteFrame(w io.Writer, b []byte) error {
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(b)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

// ReadFrame reads one length-prefixed FlatBuffers join message.
func ReadFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	if n > MaxBody {
		return nil, ErrParams
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}
