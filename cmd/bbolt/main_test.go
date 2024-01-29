package main_test

import (
	"bytes"
	crypto "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"go.etcd.io/bbolt/internal/btesting"

	"github.com/stretchr/testify/require"

	bolt "go.etcd.io/bbolt"
	main "go.etcd.io/bbolt/cmd/bbolt"
)

// Ensure the "info" command can print information about a database.
func TestInfoCommand_Run(t *testing.T) {
	db := btesting.MustCreateDB(t)
	db.Close()

	defer requireDBNoChange(t, dbData(t, db.Path()), db.Path())

	// Run the info command.
	m := NewMain()
	if err := m.Run("info", db.Path()); err != nil {
		t.Fatal(err)
	}
}

// Ensure the "stats" command executes correctly with an empty database.
func TestStatsCommand_Run_EmptyDatabase(t *testing.T) {
	// Ignore
	if os.Getpagesize() != 4096 {
		t.Skip("system does not use 4KB page size")
	}

	db := btesting.MustCreateDB(t)
	db.Close()

	defer requireDBNoChange(t, dbData(t, db.Path()), db.Path())

	// Generate expected result.
	exp := "Aggregate statistics for 0 buckets\n\n" +
		"Page count statistics\n" +
		"\tNumber of logical branch pages: 0\n" +
		"\tNumber of physical branch overflow pages: 0\n" +
		"\tNumber of logical leaf pages: 0\n" +
		"\tNumber of physical leaf overflow pages: 0\n" +
		"Tree statistics\n" +
		"\tNumber of keys/value pairs: 0\n" +
		"\tNumber of levels in B+tree: 0\n" +
		"Page size utilization\n" +
		"\tBytes allocated for physical branch pages: 0\n" +
		"\tBytes actually used for branch data: 0 (0%)\n" +
		"\tBytes allocated for physical leaf pages: 0\n" +
		"\tBytes actually used for leaf data: 0 (0%)\n" +
		"Bucket statistics\n" +
		"\tTotal number of buckets: 0\n" +
		"\tTotal number on inlined buckets: 0 (0%)\n" +
		"\tBytes used for inlined buckets: 0 (0%)\n"

	// Run the command.
	m := NewMain()
	if err := m.Run("stats", db.Path()); err != nil {
		t.Fatal(err)
	} else if m.Stdout.String() != exp {
		t.Fatalf("unexpected stdout:\n\n%s", m.Stdout.String())
	}
}

// Ensure the "stats" command can execute correctly.
func TestStatsCommand_Run(t *testing.T) {
	// Ignore
	if os.Getpagesize() != 4096 {
		t.Skip("system does not use 4KB page size")
	}

	db := btesting.MustCreateDB(t)

	if err := db.Update(func(tx *bolt.Tx) error {
		// Create "foo" bucket.
		b, err := tx.CreateBucket([]byte("foo"))
		if err != nil {
			return err
		}
		for i := 0; i < 10; i++ {
			if err := b.Put([]byte(strconv.Itoa(i)), []byte(strconv.Itoa(i))); err != nil {
				return err
			}
		}

		// Create "bar" bucket.
		b, err = tx.CreateBucket([]byte("bar"))
		if err != nil {
			return err
		}
		for i := 0; i < 100; i++ {
			if err := b.Put([]byte(strconv.Itoa(i)), []byte(strconv.Itoa(i))); err != nil {
				return err
			}
		}

		// Create "baz" bucket.
		b, err = tx.CreateBucket([]byte("baz"))
		if err != nil {
			return err
		}
		if err := b.Put([]byte("key"), []byte("value")); err != nil {
			return err
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	defer requireDBNoChange(t, dbData(t, db.Path()), db.Path())

	// Generate expected result.
	exp := "Aggregate statistics for 3 buckets\n\n" +
		"Page count statistics\n" +
		"\tNumber of logical branch pages: 0\n" +
		"\tNumber of physical branch overflow pages: 0\n" +
		"\tNumber of logical leaf pages: 1\n" +
		"\tNumber of physical leaf overflow pages: 0\n" +
		"Tree statistics\n" +
		"\tNumber of keys/value pairs: 111\n" +
		"\tNumber of levels in B+tree: 1\n" +
		"Page size utilization\n" +
		"\tBytes allocated for physical branch pages: 0\n" +
		"\tBytes actually used for branch data: 0 (0%)\n" +
		"\tBytes allocated for physical leaf pages: 4096\n" +
		"\tBytes actually used for leaf data: 1996 (48%)\n" +
		"Bucket statistics\n" +
		"\tTotal number of buckets: 3\n" +
		"\tTotal number on inlined buckets: 2 (66%)\n" +
		"\tBytes used for inlined buckets: 236 (11%)\n"

	// Run the command.
	m := NewMain()
	if err := m.Run("stats", db.Path()); err != nil {
		t.Fatal(err)
	} else if m.Stdout.String() != exp {
		t.Fatalf("unexpected stdout:\n\n%s", m.Stdout.String())
	}
}

// Ensure the "buckets" command can print a list of buckets.
func TestBucketsCommand_Run(t *testing.T) {
	db := btesting.MustCreateDB(t)

	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{"foo", "bar", "baz"} {
			_, err := tx.CreateBucket([]byte(name))
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	defer requireDBNoChange(t, dbData(t, db.Path()), db.Path())

	expected := "bar\nbaz\nfoo\n"

	// Run the command.
	m := NewMain()
	if err := m.Run("buckets", db.Path()); err != nil {
		t.Fatal(err)
	} else if actual := m.Stdout.String(); actual != expected {
		t.Fatalf("unexpected stdout:\n\n%s", actual)
	}
}

// Ensure the "keys" command can print a list of keys for a bucket.
func TestKeysCommand_Run(t *testing.T) {
	db := btesting.MustCreateDB(t)

	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{"foo", "bar"} {
			b, err := tx.CreateBucket([]byte(name))
			if err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				key := fmt.Sprintf("%s-%d", name, i)
				if err := b.Put([]byte(key), []byte{0}); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	defer requireDBNoChange(t, dbData(t, db.Path()), db.Path())

	expected := "foo-0\nfoo-1\nfoo-2\n"

	// Run the command.
	m := NewMain()
	if err := m.Run("keys", db.Path(), "foo"); err != nil {
		t.Fatal(err)
	} else if actual := m.Stdout.String(); actual != expected {
		t.Fatalf("unexpected stdout:\n\n%s", actual)
	}
}

// Ensure the "get" command can print the value of a key in a bucket.
func TestGetCommand_Run(t *testing.T) {
	db := btesting.MustCreateDB(t)

	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{"foo", "bar"} {
			b, err := tx.CreateBucket([]byte(name))
			if err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				key := fmt.Sprintf("%s-%d", name, i)
				val := fmt.Sprintf("val-%s-%d", name, i)
				if err := b.Put([]byte(key), []byte(val)); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	defer requireDBNoChange(t, dbData(t, db.Path()), db.Path())

	expected := "val-foo-1\n"

	// Run the command.
	m := NewMain()
	if err := m.Run("get", db.Path(), "foo", "foo-1"); err != nil {
		t.Fatal(err)
	} else if actual := m.Stdout.String(); actual != expected {
		t.Fatalf("unexpected stdout:\n\n%s", actual)
	}
}

// Ensure the "pages" command neither panic, nor change the db file.
func TestPagesCommand_Run(t *testing.T) {
	db := btesting.MustCreateDB(t)

	err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{"foo", "bar"} {
			b, err := tx.CreateBucket([]byte(name))
			if err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				key := fmt.Sprintf("%s-%d", name, i)
				val := fmt.Sprintf("val-%s-%d", name, i)
				if err := b.Put([]byte(key), []byte(val)); err != nil {
					return err
				}
			}
		}
		return nil
	})
	require.NoError(t, err)
	db.Close()

	defer requireDBNoChange(t, dbData(t, db.Path()), db.Path())

	// Run the command.
	m := NewMain()
	err = m.Run("pages", db.Path())
	require.NoError(t, err)
}

// Main represents a test wrapper for main.Main that records output.
type Main struct {
	*main.Main
	Stdin  bytes.Buffer
	Stdout bytes.Buffer
	Stderr bytes.Buffer
}

// NewMain returns a new instance of Main.
func NewMain() *Main {
	m := &Main{Main: main.NewMain()}
	m.Main.Stdin = &m.Stdin
	m.Main.Stdout = &m.Stdout
	m.Main.Stderr = &m.Stderr
	return m
}

func TestCompactCommand_Run(t *testing.T) {
	var s int64
	if err := binary.Read(crypto.Reader, binary.BigEndian, &s); err != nil {
		t.Fatal(err)
	}
	rand.Seed(s)

	dstdb := btesting.MustCreateDB(t)
	dstdb.Close()

	// fill the db
	db := btesting.MustCreateDB(t)
	if err := db.Update(func(tx *bolt.Tx) error {
		n := 2 + rand.Intn(5)
		for i := 0; i < n; i++ {
			k := []byte(fmt.Sprintf("b%d", i))
			b, err := tx.CreateBucketIfNotExists(k)
			if err != nil {
				return err
			}
			if err := b.SetSequence(uint64(i)); err != nil {
				return err
			}
			if err := fillBucket(b, append(k, '.')); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// make the db grow by adding large values, and delete them.
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("large_vals"))
		if err != nil {
			return err
		}
		n := 5 + rand.Intn(5)
		for i := 0; i < n; i++ {
			v := make([]byte, 1000*1000*(1+rand.Intn(5)))
			_, err := crypto.Read(v)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(fmt.Sprintf("l%d", i)), v); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte("large_vals")).Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if err := c.Delete(); err != nil {
				return err
			}
		}
		return tx.DeleteBucket([]byte("large_vals"))
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	dbChk, err := chkdb(db.Path())
	if err != nil {
		t.Fatal(err)
	}

	m := NewMain()
	if err := m.Run("compact", "-o", dstdb.Path(), db.Path()); err != nil {
		t.Fatal(err)
	}

	dbChkAfterCompact, err := chkdb(db.Path())
	if err != nil {
		t.Fatal(err)
	}

	dstdbChk, err := chkdb(dstdb.Path())
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(dbChk, dbChkAfterCompact) {
		t.Error("the original db has been touched")
	}
	if !bytes.Equal(dbChk, dstdbChk) {
		t.Error("the compacted db data isn't the same than the original db")
	}
}

func TestCommands_Run_NoArgs(t *testing.T) {
	testCases := []struct {
		name   string
		cmd    string
		expErr error
	}{
		{
			name:   "get",
			cmd:    "get",
			expErr: main.ErrNotEnoughArgs,
		},
		{
			name:   "keys",
			cmd:    "keys",
			expErr: main.ErrNotEnoughArgs,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMain()
			err := m.Run(tc.cmd)
			require.ErrorIs(t, err, main.ErrNotEnoughArgs)
		})
	}
}

func fillBucket(b *bolt.Bucket, prefix []byte) error {
	n := 10 + rand.Intn(50)
	for i := 0; i < n; i++ {
		v := make([]byte, 10*(1+rand.Intn(4)))
		_, err := crypto.Read(v)
		if err != nil {
			return err
		}
		k := append(prefix, []byte(fmt.Sprintf("k%d", i))...)
		if err := b.Put(k, v); err != nil {
			return err
		}
	}
	// limit depth of subbuckets
	s := 2 + rand.Intn(4)
	if len(prefix) > (2*s + 1) {
		return nil
	}
	n = 1 + rand.Intn(3)
	for i := 0; i < n; i++ {
		k := append(prefix, []byte(fmt.Sprintf("b%d", i))...)
		sb, err := b.CreateBucket(k)
		if err != nil {
			return err
		}
		if err := fillBucket(sb, append(k, '.')); err != nil {
			return err
		}
	}
	return nil
}

func chkdb(path string) ([]byte, error) {
	db, err := bolt.Open(path, 0666, &bolt.Options{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var buf bytes.Buffer
	err = db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			return walkBucket(b, name, nil, &buf)
		})
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func walkBucket(parent *bolt.Bucket, k []byte, v []byte, w io.Writer) error {
	if _, err := fmt.Fprintf(w, "%d:%x=%x\n", parent.Sequence(), k, v); err != nil {
		return err
	}

	// not a bucket, exit.
	if v != nil {
		return nil
	}
	return parent.ForEach(func(k, v []byte) error {
		if v == nil {
			return walkBucket(parent.Bucket(k), k, nil, w)
		}
		return walkBucket(parent, k, v, w)
	})
}

func dbData(t *testing.T, filePath string) []byte {
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	return data
}

func requireDBNoChange(t *testing.T, oldData []byte, filePath string) {
	newData, err := os.ReadFile(filePath)
	require.NoError(t, err)

	noChange := bytes.Equal(oldData, newData)
	require.True(t, noChange)
}
