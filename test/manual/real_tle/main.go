// Command real_tle creates and judges a contest-style query problem. Its large
// case has n=q=200000: upper_bound is accepted, while the direct O(nq) scan
// is a genuine algorithmic TLE rather than an artificial busy loop.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"strconv"
)

type problem struct {
	ID int64 `json:"id"`
}
type submission struct {
	ID int64 `json:"id"`
}

func main() {
	server := flag.String("server", "http://127.0.0.1:18081", "OJ server URL")
	flag.Parse()
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	client := &http.Client{Jar: jar}
	post(client, *server, "/api/login", map[string]string{"username": "alice", "password": "demo-password"}, nil)
	var item problem
	post(client, *server, "/api/problems", map[string]any{
		"title":              "有序数组计数查询",
		"statement_markdown": "给定一个非降序数组。对每个查询值 $x$，输出数组中不大于 $x$ 的元素个数。\n\n$n,q\\le 2\\times10^5$。",
		"score":              "100", "resource_limits": map[string]any{}, "compare": map[string]any{},
	}, &item)
	upload(client, *server, item.ID, archiveTests())
	post(client, *server, "/api/logout", map[string]any{}, nil)
	post(client, *server, "/api/login", map[string]string{"username": "bob", "password": "demo-password"}, nil)
	accepted := `#include <algorithm>
#include <iostream>
#include <vector>
using namespace std;
int main(){int n,q;if(!(cin>>n>>q))return 0;vector<int>a(n);for(int&i:a)cin>>i;while(q--){int x;cin>>x;cout<<upper_bound(a.begin(),a.end(),x)-a.begin()<<'\n';}}`
	brute := `#include <iostream>
#include <vector>
using namespace std;
int main(){int n,q;if(!(cin>>n>>q))return 0;vector<int>a(n);for(int&i:a)cin>>i;while(q--){int x,ans=0;cin>>x;for(int value:a)if(value<=x)++ans;cout<<ans<<'\n';}}`
	var ac, tle submission
	post(client, *server, "/api/problems/"+strconv.FormatInt(item.ID, 10)+"/submissions", map[string]string{"language": "cpp", "source": accepted}, &ac)
	post(client, *server, "/api/problems/"+strconv.FormatInt(item.ID, 10)+"/submissions", map[string]string{"language": "cpp", "source": brute}, &tle)
	fmt.Printf("problem_id=%d ac_submission_id=%d brute_submission_id=%d\n", item.ID, ac.ID, tle.ID)
}

func archiveTests() []byte {
	files := map[string][]byte{"1.in": []byte("5 3\n1 3 3 7 9\n0\n3\n10\n"), "1.out": []byte("0\n3\n5\n")}
	var input, output bytes.Buffer
	const n, q = 200000, 200000
	fmt.Fprintf(&input, "%d %d\n", n, q)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&input, "%d ", i)
	}
	input.WriteByte('\n')
	for i := 1; i <= q; i++ {
		fmt.Fprintf(&input, "%d\n", i)
		fmt.Fprintf(&output, "%d\n", i)
	}
	files["2.in"], files["2.out"] = input.Bytes(), output.Bytes()
	var result bytes.Buffer
	writer := zip.NewWriter(&result)
	for name, data := range files {
		file, err := writer.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := file.Write(data); err != nil {
			panic(err)
		}
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return result.Bytes()
}

func upload(client *http.Client, server string, id int64, data []byte) {
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	file, err := writer.CreateFormFile("file", "ordered-query-tests.zip")
	if err != nil {
		panic(err)
	}
	if _, err = file.Write(data); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	request, err := http.NewRequest(http.MethodPost, server+"/api/problems/"+strconv.FormatInt(id, 10)+"/testdata", &form)
	if err != nil {
		panic(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		panic(string(body))
	}
}
func post(client *http.Client, server, path string, input, output any) {
	data, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}
	request, err := http.NewRequest(http.MethodPost, server+path, bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}
	if response.StatusCode >= 300 {
		panic(string(body))
	}
	if output != nil {
		if err := json.Unmarshal(body, output); err != nil {
			panic(err)
		}
	}
}
