// Command seed_normal_data exercises the normal OJ workflow through its HTTP
// API. It creates accounts, two authored problems, test ZIPs, and submissions
// with AC, WA, CE, and floating-point comparison outcomes.
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

const password = "demo-password"

var baseURL string

type problem struct {
	ID int64 `json:"id"`
}

func main() {
	server := flag.String("server", "http://127.0.0.1:18081", "OJ server URL")
	flag.Parse()
	baseURL = *server
	admin := client(*server)
	login(admin, "admin", "local-test-password")
	for _, username := range []string{"alice", "bob", "coach"} {
		requestJSON(admin, http.MethodPost, "/api/accounts", map[string]string{"username": username, "password": password, "role": "user"}, nil)
	}

	alice := client(*server)
	login(alice, "alice", password)
	sum := createProblem(alice, "整数序列求和", "给定 $n$ 个整数，输出它们的和。\n\n输入第一行为 $n$，第二行包含这 $n$ 个整数。", "100", false, 0)
	uploadTests(alice, sum.ID, map[string]string{
		"1.in": "3\n1 2 3\n", "1.out": "6\n",
		"2.in": "5\n-10 20 -30 40 50\n", "2.out": "70\n",
		"3.in": "1\n1000000000\n", "3.out": "1000000000\n",
	})
	average := createProblem(alice, "浮点平均值", "给定 $n$ 个实数，输出它们的算术平均值。启用 $10^{-6}$ 绝对误差。", "100", true, 1e-6)
	uploadTests(alice, average.ID, map[string]string{
		"1.in": "3\n1 2 2\n", "1.out": "1.6666666667\n",
		"2.in": "4\n1.5 2.5 3.5 4.5\n", "2.out": "3\n",
	})

	bob := client(*server)
	login(bob, "bob", password)
	submit(bob, sum.ID, "cpp", `#include <iostream>
int main(){ long long n,x,s=0; if(!(std::cin>>n)) return 0; while(n--){std::cin>>x;s+=x;} std::cout<<s<<'\n'; }`)
	submit(bob, sum.ID, "cpp", `#include <iostream>
int main(){ std::cout << 0 << '\n'; }`)
	submit(bob, sum.ID, "cpp", `int main( { return 0; }`)
	submit(bob, average.ID, "python3", `n=int(input())
a=list(map(float,input().split()))
print(sum(a)/n + 0.0000005)`)
	submit(bob, average.ID, "python3", `n=int(input())
a=list(map(float,input().split()))
print(round(sum(a)/n, 1))`)
	fmt.Println("created normal accounts, problems, test data, and five submissions")
}

func client(server string) *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return &http.Client{Jar: jar}
}
func login(c *http.Client, username, pass string) {
	requestJSON(c, http.MethodPost, "/api/login", map[string]string{"username": username, "password": pass}, nil)
}
func createProblem(c *http.Client, title, statement, score string, floating bool, epsilon float64) problem {
	var value problem
	requestJSON(c, http.MethodPost, "/api/problems", map[string]any{"title": title, "statement_markdown": statement, "score": score, "compare": map[string]any{"floating_point": floating, "epsilon": epsilon}, "resource_limits": map[string]any{}}, &value)
	return value
}
func submit(c *http.Client, id int64, language, source string) {
	requestJSON(c, http.MethodPost, "/api/problems/"+strconv.FormatInt(id, 10)+"/submissions", map[string]string{"language": language, "source": source}, nil)
}
func uploadTests(c *http.Client, id int64, files map[string]string) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, value := range files {
		entry, err := writer.Create(name)
		must(err)
		_, err = entry.Write([]byte(value))
		must(err)
	}
	must(writer.Close())
	var form bytes.Buffer
	multipartWriter := multipart.NewWriter(&form)
	file, err := multipartWriter.CreateFormFile("file", "tests.zip")
	must(err)
	_, err = file.Write(archive.Bytes())
	must(err)
	must(multipartWriter.Close())
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/problems/"+strconv.FormatInt(id, 10)+"/testdata", &form)
	must(err)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	response, err := c.Do(request)
	must(err)
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		panic(string(body))
	}
}
func requestJSON(c *http.Client, method, path string, input any, output any) {
	data, err := json.Marshal(input)
	must(err)
	request, err := http.NewRequest(method, baseURL+path, bytes.NewReader(data))
	must(err)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.Do(request)
	must(err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	must(err)
	if response.StatusCode >= 300 {
		panic(fmt.Sprintf("%s %s: %s", method, path, body))
	}
	if output != nil {
		must(json.Unmarshal(body, output))
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
