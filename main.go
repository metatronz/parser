package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/antchfx/htmlquery"
)

type boardItem struct {
	Text string `json:"text"`
	Tel  string `json:"tel"`
}

var pages chan int
var queue chan boardItem

func main() {
	workersNum := 20
	start := time.Now()
	var wg sync.WaitGroup
	sigs := make(chan os.Signal)
	done := make(chan struct{})

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Println(sig)
		close(done)
	}()

	pages = make(chan int, 10)

	queue = make(chan boardItem, 50)

	go func() {
		for i := 0; i < 1000; i++ {
			select {
			case <-done:
				close(pages)
				println("terminate received")
				return
			default:
				pages <- i
			}
		}
		close(pages)
	}()

	for i := 0; i < workersNum; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for page := range pages {
				pageTask(page)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(queue)
	}()

	f, err := os.Create("b.json")
	if err != nil {
		log.Fatalln("error open file")
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	writer.WriteString("[\n")
	for v := range queue {
		jboard, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			log.Println("error convert")
			continue
		}
		writer.Write(jboard)
		writer.WriteString(",\n")
	}
	writer.Flush()
	f.Seek(int64(-len(",\n")), io.SeekEnd)
	fi, _ := f.Stat()
	f.Truncate(fi.Size() - int64(len(",\n")))
	writer.WriteString("\n]")
	writer.Flush()

	elapsed := time.Since(start)
	fmt.Printf("==========> %s\n", elapsed)
}

func pageTask(page int) {
	//doc, err := htmlquery.LoadURL(fmt.Sprintf("%s%d", "https://berkat.ru/board?page=", page+1))
	doc, err := htmlquery.LoadURL("https://berkat.ru/board?page=" + strconv.Itoa(page+1))
	if err != nil {
		log.Println(`Cannot load URL`)
		return
	}

	nodes, err := htmlquery.QueryAll(doc, `//div[contains(@id,"board_list_item")]`)

	if err != nil {
		log.Println(`not a valid XPath expression.`)
		return
	}
	for _, v := range nodes {
		tel := htmlquery.InnerText(htmlquery.FindOne(v, `//a[@class="get_phone_style"]`))
		text := htmlquery.InnerText(htmlquery.FindOne(v, `//p[@class="board_list_item_text"]`))
		queue <- boardItem{Text: text, Tel: tel}
	}
	log.Println("Page ", page+1)
}
