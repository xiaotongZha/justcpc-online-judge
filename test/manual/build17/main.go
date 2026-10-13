// Command build_17_package creates the temporary real-data package used by
// run-real17.sh. It is a manual test tool and is not linked into the OJ.
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	output := flag.String("output", "", "output ZIP path")
	dataRoot := flag.String("data-root", "", "17th contest data directory")
	fixtureRoot := flag.String("fixture-root", "test/fixtures", "fixture directory")
	flag.Parse()
	if *output == "" || *dataRoot == "" {
		fmt.Fprintln(os.Stderr, "-output and -data-root are required")
		os.Exit(2)
	}
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fatal(err)
	}
	archive := zip.NewWriter(file)
	defer func() {
		if err := archive.Close(); err != nil {
			fatal(err)
		}
		if err := file.Close(); err != nil {
			fatal(err)
		}
	}()

	copyFile(archive, filepath.Join(*fixtureRoot, "contest-17-sample.json"), "contest.json")
	copyFile(archive, filepath.Join(*fixtureRoot, "contest-17-sample-contestants.csv"), "contestants.csv")
	copyFile(archive, filepath.Join(*dataRoot, "第十七届江苏科技大学程序设计竞赛(正式赛).pdf"), "statement.pdf")
	for _, label := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		entries, err := os.ReadDir(filepath.Join(*dataRoot, "test_data", label))
		if err != nil {
			fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".in" && filepath.Ext(entry.Name()) != ".out" {
				continue
			}
			copyFile(archive, filepath.Join(*dataRoot, "test_data", label, entry.Name()), filepath.Join("tests", label, entry.Name()))
		}
	}
	addSources(archive, filepath.Join(*dataRoot, "code-JustCPC", "code-cpp"), "222210712113", ".cpp")
	addSources(archive, filepath.Join(*dataRoot, "code-JustCPC", "code-cpp"), "222210714224", ".cpp")
	addSources(archive, filepath.Join(*dataRoot, "code-JustCPC", "code-py"), "242210788536", ".py")
	addSources(archive, filepath.Join(*dataRoot, "code-JustCPC", "code-py"), "232241802140", ".py")
	addSources(archive, filepath.Join(*dataRoot, "code-JustCPC", "code-java", "batch3"), "232241803926", ".java")
}

func addSources(archive *zip.Writer, root, student, extension string) {
	entries, err := os.ReadDir(filepath.Join(root, student))
	if err != nil {
		fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != extension {
			continue
		}
		label := entry.Name()[:len(entry.Name())-len(extension)]
		if label == "H" { // The 17th contest has no valid H submissions.
			continue
		}
		copyFile(archive, filepath.Join(root, student, entry.Name()), filepath.Join("submissions", student, label, "source"+extension))
	}
}

func copyFile(archive *zip.Writer, source, name string) {
	input, err := os.Open(source)
	if err != nil {
		fatal(err)
	}
	defer input.Close()
	output, err := archive.Create(name)
	if err != nil {
		fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
