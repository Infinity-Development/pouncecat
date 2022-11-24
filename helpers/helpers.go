// Provides some helper functions so you don't have to write them yourself
package helpers

import (
	"io"
	"math/rand"
	"net/http"
	"pouncecat/ui"
	"time"
	"unsafe"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandString(n int) string {
	// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go

	var src = rand.NewSource(time.Now().UnixNano())

	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}

func PromptServerChannel(message string) string {
	ui.NotifyMsg("info", "To continue, please send an input to the following question to http://localhost:34012/msg: "+message)
	channel := make(chan string)

	killChan := make(chan bool)

	go func() {
		r := http.NewServeMux()

		srv := &http.Server{Addr: ":34012", Handler: r}

		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(message))
		})

		r.HandleFunc("/msg", func(w http.ResponseWriter, r *http.Request) {
			// Read body
			body, err := io.ReadAll(r.Body)

			if err != nil {
				w.Write([]byte("Error reading body"))
				return
			}

			channel <- string(body)
		})

		go srv.ListenAndServe()

		<-killChan

		ui.NotifyMsg("info", "Closing server")

		srv.Close()
	}()

	id := <-channel

	ui.NotifyMsg("info", "Received input: "+id)

	killChan <- true

	return id
}
