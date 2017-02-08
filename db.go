package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/cheggaaa/pb"
)

func getQuotesFromIndex(index string) []Quote {
	index = cleanString(index)

	db, err := bolt.Open("quotations.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	indexMap := make(map[int]bool)
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("index"))
		v := b.Get([]byte(index))
		if v != nil {
			json.Unmarshal(v, &indexMap)
		}
		return nil
	})
	quotes := make([]Quote, len(indexMap))

	var quote Quote
	i := 0
	for id := range indexMap {
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("data"))
			v := b.Get(itob(id))
			json.Unmarshal(v, &quote)
			quotes[i] = quote
			i++
			return nil
		})
	}

	if len(quotes) == 0 {
		log.Println("Could not find in index, going to iterate through everything")
		indexMap = make(map[int]bool)
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("data"))
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				json.Unmarshal(v, &quote)
				if strings.Contains(strings.ToLower(quote.Text), index) ||
					strings.Contains(strings.ToLower(quote.Name), index) {
					quotes = append(quotes, quote)
					indexMap[quote.ID] = true
				}
			}
			return nil
		})

		if len(indexMap) > 0 {
			// Insert into the index, now that something is found
			db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("index"))
				buf, err := json.Marshal(indexMap)
				if err != nil {
					return err
				}
				b.Put([]byte(index), buf)
				return nil
			})
		}
	}

	log.Printf("Found %d quotes for '%s'", len(quotes), index)
	return quotes
}

var randomQuotePool = struct {
	sync.RWMutex
	q []Quote
}{q: make([]Quote, 0)}

func getRandomQuotes(num int) []Quote {
	if num < 1 {
		num = 1
	} else if num > 50 {
		num = 50
	}
	var quotes []Quote
	randomQuotePool.RLock()
	if len(randomQuotePool.q) > num {
		quotes = randomQuotePool.q[0:num]
	} else {
		quotes = generateRandomQuotes(num)
	}
	randomQuotePool.RUnlock()
	go cacheRandomQuotes(num)
	return quotes
}

func cacheRandomQuotes(num int) {
	randomQuotePool.Lock()
	if len(randomQuotePool.q) == 0 {
		randomQuotePool.q = generateRandomQuotes(2000)
	} else {
		randomQuotePool.q = randomQuotePool.q[num:]
		randomQuotePool.q = append(randomQuotePool.q, generateRandomQuotes(num)...)
	}
	randomQuotePool.Unlock()
}

func generateRandomQuotes(num int) []Quote {
	quotes := make([]Quote, num)
	db, err := bolt.Open("quotations.db", 0600, nil)
	if err != nil {
		log.Printf("Error: %v\n", err)
		db.Close()
		return quotes
	}
	defer db.Close()

	var quote Quote
	maxNumber := 0
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("data"))
		c := b.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			json.Unmarshal(v, &quote)
			maxNumber = quote.ID
			return nil
		}
		return nil
	})

	for i := range quotes {
		randomID := rand.Intn(maxNumber)
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("data"))
			c := b.Cursor()
			_, v := c.Seek(itob(randomID))
			json.Unmarshal(v, &quote)
			quotes[i] = quote
			return nil
		})
	}

	return quotes
}

func dumpDatabase() {
	fmt.Println("Dumping database...")

	db, err := bolt.Open("quotations.db", 0600, nil)
	if err != nil {
		log.Printf("Error: %v\n", err)
		db.Close()
	}
	defer db.Close()

	var quote Quote
	maxNumber := 0
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("data"))
		c := b.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			json.Unmarshal(v, &quote)
			maxNumber = quote.ID
			return nil
		}
		return nil
	})

	quotes := make([]Quote, maxNumber)
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("data"))
		c := b.Cursor()
		i := 0
		for k, v := c.First(); k != nil; k, v = c.Next() {
			json.Unmarshal(v, &quote)
			quotes[i] = quote
			i++
		}
		return nil
	})

	bJson, _ := json.MarshalIndent(quotes, "", " ")
	ioutil.WriteFile("quotations.json", bJson, 0644)
}

func buildDatabase() {
	fmt.Println("Building database...")
	var quotes []Quote
	bJson, _ := ioutil.ReadFile("quotations.json")
	json.Unmarshal(bJson, &quotes)

	db, err := bolt.Open("quotations.db", 0600, nil)
	if err != nil {
		log.Printf("Error: %v\n", err)
		db.Close()
		return
	}
	defer db.Close()

	fmt.Println("Inserting data")
	indexing := make(map[string]map[int]bool)
	bar := pb.StartNew(len(quotes))
	alreadyFinished := make(map[string]bool)
	db.Update(func(tx *bolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte("data"))
		for _, quote := range quotes {
			bar.Increment()

			// Skip quotes that are too long
			if len(quote.Text) > 300 {
				continue
			}
			// Skip quotes that have already been added
			h := sha1.New()
			h.Write([]byte(cleanString(quote.Text)))
			sha1_hash := hex.EncodeToString(h.Sum(nil))
			if _, ok := alreadyFinished[sha1_hash]; ok {
				continue
			}
			alreadyFinished[sha1_hash] = true

			// Add this quote!
			id, _ := b.NextSequence()
			quote.ID = int(id)
			quote.Name = strings.TrimSpace(quote.Name)
			quote.Text = strings.TrimSpace(quote.Text)

			buf, err := json.Marshal(quote)
			if err != nil {
				return err
			}
			b.Put(itob(quote.ID), buf)

			// Do the indexing of the words
			for _, word := range strings.Fields(quote.Text) {
				word = cleanString(word)
				// Only index if its not a stop word
				if !isStopWord(word) {
					if _, ok := indexing[word]; !ok {
						indexing[word] = make(map[int]bool)
					}
					if _, ok := indexing[word][quote.ID]; !ok {
						indexing[word][quote.ID] = true
					}
				}
			}

			// Do the indexing of the authors
			word := cleanString(quote.Name)
			// Only index if its not a stop word
			if _, ok := indexing[word]; !ok {
				indexing[word] = make(map[int]bool)
			}
			if _, ok := indexing[word][quote.ID]; !ok {
				indexing[word][quote.ID] = true
			}

			// continue looping through quotes!
		}
		return nil
	})
	bar.FinishPrint("Finished inserting data")

	fmt.Println("Indexing data")
	bar = pb.StartNew(len(indexing))
	db.Update(func(tx *bolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte("index"))
		for word := range indexing {
			bar.Increment()
			buf, err := json.Marshal(indexing[word])
			if err != nil {
				return err
			}
			b.Put([]byte(word), buf)
		}
		return nil
	})
	bar.FinishPrint("Finished indexing data")
}

// itob returns an 8-byte big endian representation of v.
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}
