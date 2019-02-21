package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"hash"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/obourdon/magicmime"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// flag Example 1: A single string flag called "species" with default value "gopher".
// var rootdir = flag.String("rootdir", ".", "the top `directory` to be parsed")

// flag Example 2: Two flags sharing a variable, so we can have a shorthand.
// The order of initialization is undefined, so make sure both use the
// same default value. They must be set up with an init function.
var rootdir string
var dbfile string
var notdatesuffixed bool

// Walker structure for referencing SQLite DB during file tree traversal
type Walker struct {
	Db      *gorm.DB
	RootDir string
	Now     int64
}

type Files struct {
	gorm.Model
	//Id int `gorm:"PRIMARY_KEY;AUTO_INCREMENT;NOT NULL"`

	Path string `gorm:"NOT NULL"`
	Dir  string `gorm:"NOT NULL"`
	File string `gorm:"NOT NULL"`
	Ext  string

	Inserted int64 `gorm:"NOT NULL"`
	Deleted  int64
	Lastseen int64 `gorm:"NOT NULL"`

	Modified int64 `gorm:"NOT NULL"`
	Changed  int64 `gorm:"NOT NULL"`
	Accessed int64 `gorm:"NOT NULL"`
	Created  int64

	Size  int64  `gorm:"NOT NULL"`
	Mode  uint32 `gorm:"NOT NULL"`
	Uid   uint32 `gorm:"NOT NULL"`
	Gid   uint32 `gorm:"NOT NULL"`
	Dev   int32
	Links uint16

	MD5Sum        string
	SHA256Sum     string
	MagicMimeType string
	MagicCharset  string
	MimeType      string
	MimeCharset   string
	ExtMimeType   string
	Test          string
}

// Hasher structure containing Files structure field to be assigned and hashing function
// as well as converter function
type Hasher struct {
	field  string
	hasher hash.Hash
	converter func([]byte)string
}

func init() {
	const (
		defaultRoot            = "."
		rootUsage              = "the top `directory` to be parsed"
		defaultDB              = "./files.db"
		DBUsage                = "the SQLite database `file`"
		defaultNotDateSuffixed = false
		NotDateSuffixUsage     = "don't use date suffix in the form YYYYMMDD for SQLite database file"
	)
	flag.StringVar(&rootdir, "rootdir", defaultRoot, rootUsage)
	flag.StringVar(&rootdir, "r", defaultRoot, rootUsage+" (shorthand)")
	flag.StringVar(&dbfile, "db", defaultDB, DBUsage)
	flag.StringVar(&dbfile, "d", defaultDB, DBUsage+" (shorthand)")
	flag.BoolVar(&notdatesuffixed, "D", defaultNotDateSuffixed, NotDateSuffixUsage)
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func GetFileContentType(out *os.File) (string, error) {

	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err := out.Read(buffer)
	if err != nil {
		return "", err
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	return contentType, nil
}

func CheckSumHash(retFile *Files, rd io.Reader, hashes ...Hasher) error {
	// Build array of hash functions
	hash_funcs := make([]io.Writer, len(hashes))
	for idx, h := range hashes {
		hash_funcs[idx] = h.hasher
	}

	// For optimum speed, Getpagesize returns the underlying system's memory page size.
	//pagesize := os.Getpagesize()

	// wraps the Reader object into a new buffered reader to read the files in chunks
	// and buffering them for performance.
	//reader := bufio.NewReaderSize(rd, pagesize)

	// creates a multiplexer Writer object that will duplicate all write
	// operations when copying data from source into all different hashing algorithms
	// at the same time
	multiWriter := io.MultiWriter(hash_funcs...)

	// Using a buffered reader, this will write to the writer multiplexer
	// so we only traverse through the file once, and can calculate all hashes
	// in a single byte buffered scan pass.
	//
	_, err := io.Copy(multiWriter, rd)
	if err != nil {
		panic(err.Error())
	}

	// Assign return of each hash function to appropriate structure field
	for idx, h := range hashes {
		v := reflect.ValueOf(retFile).Elem().FieldByName(h.field)
		if v.IsValid() {
			v.SetString(hashes[idx].converter((hash_funcs[idx].(hash.Hash)).Sum(nil)))
		}
	}
	return nil
}

func upperHex(content []byte) string {
	return strings.ToUpper(hex.EncodeToString(content))
}

// File entry visiting function
func (w *Walker) Visit(path string, f os.FileInfo, err error) error {
	// Get dirname and basename/filename
	ldir, lfile := filepath.Split(path)
	// Get extension (empty string if none) and convert to lower case
	lext := strings.ToLower(filepath.Ext(lfile))
	// Basically determining file type by extension might be tricky/wrong
	// however for some files like epub, mobi, bz2, ... where HTTP mime type gives
	// application/octet-stream this might be usefull
	// even works for txt, odp, js, html, ...
	extmime := mime.TypeByExtension(lext)
	lmimetype, err := magicmime.TypeByFile(path)
	checkErr(err)
	lmimeinfos := strings.Split(lmimetype, "; ")
	magicmimetype := lmimeinfos[0]
	magiccharset := ""
	if len(lmimeinfos) > 1 {
		magiccharset = lmimeinfos[1]
	}
	// Retrieve full (l)stat info struct
	stat, ok := f.Sys().(*syscall.Stat_t)
	// Should never happen
	if !ok {
		panic(fmt.Sprintf("Not a stat_t: %s => %v", path, f.Sys()))
	}
	// fmt.Printf("Stat: %#v\n", stat)
	// Modified, Accessed and Changed timestamps
	mtime := f.ModTime().Unix()
	atime, _ := stat.Atimespec.Unix()
	ctime, _ := stat.Ctimespec.Unix()
	// Use reflection to see if Birthtimespec field exists in struct
	s := reflect.Indirect(reflect.ValueOf(stat))
	f1 := s.FieldByName("Birthtimespec")
	// Basic file structure filled in
	newFile := &Files{
		Path:			path,
		Dir:			ldir,
		File:			lfile,
		Ext:			lext,
		Inserted:		time.Now().Unix(),
		Lastseen:		w.Now,
		Modified:		mtime,
		Changed:		ctime,
		Accessed:		atime,
		Size:			f.Size(),
		Mode:			uint32(f.Mode()),
		Uid:			stat.Uid,
		Gid:			stat.Gid,
		Dev:			stat.Dev,
		Links:			stat.Nlink,
		MagicMimeType:	magicmimetype,
		MagicCharset:	magiccharset,
		ExtMimeType:	extmime,
	}
	// Add created time if this exists
	if f1.IsValid() {
		btime, _ := stat.Birthtimespec.Unix()
		newFile.Created = btime
	}
	// Checksums for regular files only
	if (f.Mode() & os.ModeType) == 0 {
		hashes := []Hasher{
			{field: "MD5Sum", hasher: md5.New(), converter: hex.EncodeToString},
			{field: "SHA256Sum", hasher: sha256.New(), converter: hex.EncodeToString},
			{field: "Test", hasher: sha256.New(), converter: upperHex},
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		err1 := CheckSumHash(newFile, f, hashes...)
		checkErr(err1)
		// Rewind file
		f.Seek(0, 0)
		contentType, err2 := GetFileContentType(f)
		if err2 == nil {
			lcontentinfos := strings.Split(contentType, "; ")
			newFile.MimeType = lcontentinfos[0]
			if len(lcontentinfos) > 1 {
				newFile.MimeCharset = lcontentinfos[1]
			}
		}
	}
	// Insert into DB, fields can then be retrieved with the following statements
	//
	// select printf("%o",mode) from files;
	//  select datetime(modified, 'unixepoch'),datetime(changed, 'unixepoch'),datetime(accessed, 'unixepoch'),datetime(created, 'unixepoch') from files;
	// select *,path,size,printf("%o",mode),datetime(modified, 'unixepoch'),datetime(created, 'unixepoch') from files where file='ScanDir';
	w.Db.Create(newFile)
	return nil
}

func main() {
	// All the interesting pieces are with the variables declared above, but
	// to enable the flag package to see the flags defined there, one must
	// execute, typically at the start of main (not init!):
	flag.Parse()
	// Remaining command line arguments
	if len(flag.Args()) > 0 {
		panic(fmt.Sprintln("Unparsed: ", flag.Args()))
	}
	// Were some flags set ?
	// fmt.Println("Flags:", flag.NFlag())
	// Were some args set ?
	// fmt.Println("Args:", flag.NArg())
	fmt.Println("Parsing rootdir:", rootdir)

	// can add magicmime.MAGIC_ERROR | magicmime.MAGIC_CONTINUE too
	if err := magicmime.Open(magicmime.MAGIC_MIME); err != nil {
		checkErr(err)
	}
	defer magicmime.Close()

	if !notdatesuffixed {
		// YYYYMMDD format
		date_suffix := strings.Replace(strings.Split(time.Now().Format(time.RFC3339), "T")[0], "-", "", -1)
		absfile, err := filepath.Abs(dbfile)
		checkErr(err)
		dir, file := filepath.Split(absfile)
		ext := filepath.Ext(dbfile)
		rexp := regexp.MustCompile(ext + "$")
		checkErr(err)
		dbfile = filepath.Join(dir, rexp.ReplaceAllString(file, "")+"-"+date_suffix+ext)
	}

	db, err := gorm.Open("sqlite3", dbfile)
	checkErr(err)
	defer db.Close()

	// Migrate the schema
	db.AutoMigrate(&Files{})

	// tbls, err := db.Exec("PRAGMA table_info('files')")
	// tbls, err := db.Query("SELECT sql FROM sqlite_master WHERE name='files'")
	absRootdir, err := filepath.Abs(rootdir)
	checkErr(err)

	// Initialize walk structure to be passed down
	// db handle and scan start date
	w := &Walker{
		Db:      db,
		RootDir: absRootdir,
		Now:     time.Now().Unix(),
	}

	err = filepath.Walk(absRootdir, w.Visit)
	checkErr(err)

	db.Close()
}
