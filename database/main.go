package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

var (
	db *sql.DB
)

func main() {
	var err error
	db, err = sql.Open("postgres",
		"host=localhost port=5432 user=imageuser password=imagepass dbname=imagedb sslmode=disable")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	rows_images, err := db.Query("SELECT id, original_path FROM images")
	if err != nil {
		log.Fatal("Query failed:", err)
	}
	defer rows_images.Close()

	rows_processed, err := db.Query("SELECT id, processed_path FROM processed_images")
	if err != nil {
		log.Fatal("Query failed:", err)
	}
	defer rows_processed.Close()

	var id, pth string

	switch os.Args[1] {
	case "refresh":
		for rows_images.Next() {
			err = rows_images.Scan(&id, &pth)
			if err != nil {
				log.Println("Row scan failed:", err)
				continue
			}

			if !fileExists(pth) {
				_, err := db.Exec("DELETE FROM images WHERE id = $1", id)
				if err != nil {
					log.Fatal("Delete failed:", err)
				}
			}
		}

		for rows_processed.Next() {
			err = rows_processed.Scan(&id, &pth)
			if err != nil {
				log.Println("Row scan failed:", err)
				continue
			}

			if !fileExists(pth) {
				_, err := db.Exec("DELETE FROM processed_images WHERE id = $1", id)
				if err != nil {
					log.Fatal("Delete failed:", err)
				}
			}
		}
	}
}
