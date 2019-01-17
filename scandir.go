package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Example 1: A single string flag called "species" with default value "gopher".
// var rootdir = flag.String("rootdir", ".", "the top `directory` to be parsed")

// Example 2: Two flags sharing a variable, so we can have a shorthand.
// The order of initialization is undefined, so make sure both use the
// same default value. They must be set up with an init function.
var rootdir string
var dbfile string

// Walker structure for referencing SQLite DB during file tree traversal
type Walker struct {
	Db  *sql.DB
	Now int64
}

func init() {
	const (
		defaultRoot = "."
		rootUsage   = "the top `directory` to be parsed"
		defaultDB   = "./files.db"
		DBUsage     = "the SQLite database `file`"
	)
	flag.StringVar(&rootdir, "rootdir", defaultRoot, rootUsage)
	flag.StringVar(&rootdir, "r", defaultRoot, rootUsage+" (shorthand)")
	flag.StringVar(&dbfile, "db", defaultDB, DBUsage)
	flag.StringVar(&dbfile, "d", defaultDB, DBUsage+" (shorthand)")
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func (w *Walker) Visit(path string, f os.FileInfo, err error) error {
	// Get dirname and basename/filename
	ldir, lfile := filepath.Split(path)
	// Get extension (empty string if none)
	lext := filepath.Ext(lfile)
	// Retrieve full (l)stat info struct
	stat, ok := f.Sys().(*syscall.Stat_t)
	// Should never happen
	if !ok {
		panic(fmt.Sprintf("Not a stat_t: %s => %v", path, f.Sys()))
	}
	// fmt.Printf("Stat: %#v\n", stat)
	// Accessed and Changed timestamps
	atime, _ := stat.Atimespec.Unix()
	ctime, _ := stat.Ctimespec.Unix()
	// Use reflection to see if Birthtimespec field exists in struct
	s := reflect.Indirect(reflect.ValueOf(stat))
	f1 := s.FieldByName("Birthtimespec")
	//btime := int64(-1)
	btime := int64(-1)
	if f1.IsValid() {
		btime, _ = stat.Birthtimespec.Unix()
	}

	// Prepare insertion statement
	stmt, err := w.Db.Prepare("INSERT INTO files VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	checkErr(err)

	// Insert into DB, fields can then be retrieved with the following statements
	//
	// select printf("%o",mode) from files;
	//  select datetime(modified, 'unixepoch'),datetime(changed, 'unixepoch'),datetime(accessed, 'unixepoch'),datetime(created, 'unixepoch') from files;
	_, err = stmt.Exec(path, ldir, lfile, lext, w.Now, sql.NullString{}, w.Now, f.ModTime().Unix(), ctime, atime, btime, f.Size(), f.Mode(), stat.Uid, stat.Gid, stat.Dev, stat.Nlink)
	checkErr(err)

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

	db, err := sql.Open("sqlite3", dbfile)
	checkErr(err)

	files_tbl_create_stmt := "CREATE TABLE files (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, " +
		"path TXT NOT NULL, dir TXT NOT NULL, file TXT NOT NULL, ext TXT, " +
		"inserted INTEGER NOT NULL, deleted INTEGER, last_seen INTEGER NOT NULL, " +
		"modified INTEGER, changed INTEGER, accessed INTEGER, created INTEGER, " +
		"size INTEGER NOT NULL, mode INTEGER NOT NULL, uid INTEGER NOT NULL, gid INTEGER NOT NULL, dev INTEGER, links INTEGER" +
		")"
	// tbls, err := db.Exec("PRAGMA table_info('files')")
	create_tables := true
	tbls, err := db.Query("SELECT sql FROM sqlite_master WHERE name='files'")
	checkErr(err)
	var tbl_def string
	for tbls.Next() {
		err = tbls.Scan(&tbl_def)
		if strings.ToLower(tbl_def) != strings.ToLower(files_tbl_create_stmt) {
			panic("files table definition do not match")
		} else {
			create_tables = false
		}
		// fmt.Printf("Tables %#v\n", tbl_def)
	}

	// No need to access DB to get current timestamp
	// Furthermore it would require Now type to be string above
	// or converted to int64 hereafter
	//
	/* now, err := db.Query("SELECT STRFTIME('%s','NOW');")
	checkErr(err)
	var now_timestamp string
	for now.Next() {
		err = now.Scan(&now_timestamp)
		// fmt.Printf("Now %#v\n", now_timestamp)
	}
	fmt.Printf("Now %#v %#v\n", now_timestamp, time.Now().Unix()) */

	if create_tables {
		_, err = db.Exec(files_tbl_create_stmt)
		checkErr(err)
	}

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
