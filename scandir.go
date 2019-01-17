package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"time"

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
var datesuffixed bool

// Walker structure for referencing SQLite DB during file tree traversal
type Walker struct {
	Db  *gorm.DB
	Now int64
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
}

func init() {
	const (
		defaultRoot         = "."
		rootUsage           = "the top `directory` to be parsed"
		defaultDB           = "./files.db"
		DBUsage             = "the SQLite database `file`"
		defaultDateSuffixed = false
		DateSuffixUsage     = "use date suffix in the form YYYYMMDD"
	)
	flag.StringVar(&rootdir, "rootdir", defaultRoot, rootUsage)
	flag.StringVar(&rootdir, "r", defaultRoot, rootUsage+" (shorthand)")
	flag.StringVar(&dbfile, "db", defaultDB, DBUsage)
	flag.StringVar(&dbfile, "d", defaultDB, DBUsage+" (shorthand)")
	flag.BoolVar(&datesuffixed, "D", defaultDateSuffixed, DateSuffixUsage)
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func (w *Walker) Visit(path string, f os.FileInfo, err error) error {
	// Get dirname and basename/filename
	ldir, lfile := filepath.Split(path)
	// Get extension (empty string if none) and convert to lower case
	lext := strings.ToLower(filepath.Ext(lfile))
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
	// Insert into DB, fields can then be retrieved with the following statements
	//
	// select printf("%o",mode) from files;
	//  select datetime(modified, 'unixepoch'),datetime(changed, 'unixepoch'),datetime(accessed, 'unixepoch'),datetime(created, 'unixepoch') from files;
	// select *,path,size,printf("%o",mode),datetime(modified, 'unixepoch'),datetime(created, 'unixepoch') from files where file='ScanDir';
	if f1.IsValid() {
		btime, _ := stat.Birthtimespec.Unix()
		w.Db.Create(&Files{
			Path:     path,
			Dir:      ldir,
			File:     lfile,
			Ext:      lext,
			Inserted: w.Now,
			Lastseen: w.Now,
			Modified: mtime,
			Changed:  ctime,
			Accessed: atime,
			Created:  btime,
			Size:     f.Size(),
			Mode:     uint32(f.Mode()),
			Uid:      stat.Uid,
			Gid:      stat.Gid,
			Dev:      stat.Dev,
			Links:    stat.Nlink,
		})
	} else {
		w.Db.Create(&Files{
			Path:     path,
			Dir:      ldir,
			File:     lfile,
			Ext:      lext,
			Inserted: w.Now,
			Lastseen: w.Now,
			Modified: mtime,
			Changed:  ctime,
			Accessed: atime,
			Size:     f.Size(),
			Mode:     uint32(f.Mode()),
			Uid:      stat.Uid,
			Gid:      stat.Gid,
			Dev:      stat.Dev,
			Links:    stat.Nlink,
		})
	}
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

	// YYYYMMDD format
	date_suffix := strings.Replace(strings.Split(time.Now().Format(time.RFC3339), "T")[0], "-", "", -1)
	if datesuffixed {
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
	w := &Walker{
		Db:  db,
		Now: time.Now().Unix(),
	}

	absRootdir, err := filepath.Abs(rootdir)
	checkErr(err)

	err = filepath.Walk(absRootdir, w.Visit)
	checkErr(err)

	db.Close()
}
