package main

import (
	"log"
	"net/http"
)

func main() {
	// log.Print("1")
	// // init database
	// db, err := sql.Open("sqlite3", "./database/database.db")
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	// defer db.Close()

	// init server
	var fileServer http.Handler = http.FileServer(http.Dir("./source"))

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fileServer.ServeHTTP(w, r)
	})

	// start server
	log.Fatal(http.ListenAndServe(":8080", nil))
}
