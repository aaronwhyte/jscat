package main

import (
	"bufio"
	"crypto/sha1"
	"flag"
	"fmt"
	"golang.org/x/net/html"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func getHeadNode(doc *html.Node) *html.Node {
	var f func(*html.Node) *html.Node
	f = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "head" {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			h := f(c)
			if h != nil {
				return h
			}
		}
		return nil
	}
	return f(doc)
}

func getScriptNodes(doc *html.Node) []*html.Node {
	// Collect the head node's script nodes
	scriptNodes := make([]*html.Node, 0, 100)
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" && n.Parent.Data == "head" {
			scriptNodes = append(scriptNodes, n)
		} else {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
	}
	f(doc)
	return scriptNodes
}

func jsSource(scriptUrl string, htmlDir string, rootDir string) string {
	p := filepath.Join(htmlDir, scriptUrl)
	dat, err := ioutil.ReadFile(p)
	check(err)
	return string(dat)
}

func createScriptNodeWithSrc(s string) *html.Node {
	doc, _ := html.Parse(strings.NewReader("<script src=\"" + s + "\"></script>"))
	nodes := getScriptNodes(doc)
	scriptNode := nodes[0]
	scriptNode.Parent.RemoveChild(scriptNode)
	return scriptNode
}

func processHtml(srcRoot string, destRoot string, htmlPath string) {
	// Parse the HTML nodes.
	htmlFile, err := os.Open(htmlPath)
	check(err)
	doc, err := html.Parse(htmlFile)
	check(err)

	headNode := getHeadNode(doc)
	scriptNodes := getScriptNodes(headNode)
	jsSources := make([]string, len(scriptNodes))
	htmlDir := filepath.Dir(htmlPath)
	for _, n := range scriptNodes {
		for _, attr := range n.Attr {
			if attr.Key == "src" {
				scriptPath := attr.Val
				jsSources = append(jsSources, jsSource(scriptPath, htmlDir, srcRoot))
			}
		}
		// Remove the script node, and trailing EOLNs
		n2 := n.NextSibling
		if n2.Type == html.TextNode {
			n2.Data = strings.Replace(n2.Data, "\n", "", -1)
		}
		n.Parent.RemoveChild(n)
	}

	// Get the jsBytes and their SHA-1 fingerprint.
	catSource := strings.Join(jsSources, "\n\n\n")
	jsBytes := []byte(catSource)
	fingerprint := fmt.Sprintf("%x", sha1.Sum(jsBytes))

	// Insert the new script tag, with the new JS file name
	jsFileName := fingerprint + ".js"
	headNode.AppendChild(createScriptNodeWithSrc(jsFileName))

	// Calculate the destination dir for the HTML and JS, and create it if needed
	destDir, err := filepath.Rel(srcRoot, filepath.Dir(htmlPath))
	check(err)
	destDir = filepath.Join(destRoot, destDir)
	check(os.MkdirAll(destDir, 0777))

	// Write the JS file.
	jsDestPath := filepath.Join(destDir, jsFileName)
	fmt.Println("writing JS:  ", jsDestPath)
	err = ioutil.WriteFile(jsDestPath, jsBytes, 0644)
	check(err)

	// Write the HTML file.
	htmlDestPath := filepath.Join(destDir, filepath.Base(htmlPath))
	fmt.Println("writing HTML:", htmlDestPath)
	f, err := os.Create(htmlDestPath)
	check(err)
	defer f.Close()
	w := bufio.NewWriter(f)
	err = html.Render(w, doc)
	w.Flush()
}

// for each node
//     if it is a <script src="foo.js"></script> tag in the <head> section:
//       Open the JS file using the srcroot and the script tag's src attrib.
//       Append that JS file's contents to the in-mem catted script.
//       Remove the script tag's nodes.
// Get the SHA1 fingerprint (like abc123) of the final catted script.
// Insert a <head> child <script src="adb123.js"></script>
// Write the script content to abc123.js in the dest tree.
// Write the HTML file to the dest tree
func main() {
	//	htmlPathFlag := flag.String("htmlpath", "", "The file path to the HTML file with script tags")
	srcRootFlag := flag.String("srcroot", "ERROR", "The root of the source file tree")
	destRootFlag := flag.String("destroot", "ERROR", "The root of the re-written file tree")
	flag.Parse()

	if *srcRootFlag == "ERROR" || *destRootFlag == "ERROR" {
		panic("Forgot srcRootFlag or destRootFlag?")
	}
	// Make all paths absolute
	srcRoot, err := filepath.Abs(*srcRootFlag)
	check(err)
	destRoot, err := filepath.Abs(*destRootFlag)
	check(err)

	htmlPaths := os.Args[3:]
	for _, htmlPath := range htmlPaths {
		htmlPath, err := filepath.Abs(htmlPath)
		fmt.Println("reading", htmlPath)
		check(err)
		processHtml(srcRoot, destRoot, htmlPath)
		fmt.Println()
	}
}
