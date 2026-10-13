// Command edgecases submits real RE, TLE, and MLE programs to a normal OJ
// problem. It exercises the public submission API rather than writing rows.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
)

func main() {
	server := flag.String("server", "http://127.0.0.1:18081", "OJ server URL")
	problemID := flag.Int("problem", 1, "normal problem ID with 1s/256MB C++ limit")
	onlyTLE := flag.Bool("only-tle", false, "submit only the TLE program")
	flag.Parse()
	jar, err := cookiejar.New(nil)
	if err != nil { panic(err) }
	client := &http.Client{Jar: jar}
	post(client, *server+"/api/login", map[string]string{"username":"bob", "password":"demo-password"})
	sources := []string{
		`#include <csignal>
int main(){ std::raise(SIGSEGV); }`,
		`int main(){ volatile unsigned long long x=0; for(unsigned long long i=0;i<10000000000ULL;i++) x+=i; }`,
		`#include <vector>
int main(){ std::vector<int> a(80000000, 1); volatile int x=a[79999999]; (void)x; }`,
	}
	if *onlyTLE {
		sources = sources[1:2]
	}
	for _, source := range sources {
		post(client, fmt.Sprintf("%s/api/problems/%d/submissions", *server, *problemID), map[string]string{"language":"cpp", "source":source})
	}
}

func post(client *http.Client, url string, value any) {
	body, err := json.Marshal(value)
	if err != nil { panic(err) }
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil { panic(err) }
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil { panic(err) }
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil { panic(err) }
	if response.StatusCode >= 300 { panic(string(data)) }
}
