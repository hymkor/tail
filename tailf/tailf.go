package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

const DBG = true

func reader2channel(r io.Reader, ch chan string) {
	scnr := bufio.NewScanner(r)
	for scnr.Scan() {
		ch <- scnr.Text()
	}
	close(ch)
}

type Tail struct {
	Input    chan string
	LastLnum int
	Follow   bool
}

func NewTail(r io.Reader) *Tail {
	in := make(chan string, 10)
	go reader2channel(r, in)

	return &Tail{
		Input:    in,
		LastLnum: 0,
		Follow:   false,
	}
}

func (this *Tail) Run(w io.Writer, nlines int) (eof bool) {
	buffer := make([]string, nlines)
	start := this.LastLnum
	baseCtx := context.Background()
	ctx, cancel := context.WithTimeout(baseCtx, time.Second)
	stop := this.LastLnum
	for {
		select {
		case line, ok := <-this.Input:
			cancel()
			if ok { // Not Found EOF and Continue
				buffer[this.LastLnum%nlines] = line
				this.LastLnum++
				break
			} else { // Found EOF
				eof = true
				goto exit
			}
		case <-ctx.Done(): // Timeout
			if stop == this.LastLnum || this.Follow {
				// no lines read since last timeout
				eof = false
				goto exit
			} else {
				// refresh Timeout
				ctx, cancel = context.WithTimeout(baseCtx, time.Second)
				stop = this.LastLnum
			}
		}
	}
exit:
	if this.LastLnum-nlines > start {
		start = this.LastLnum - nlines
	}
	for ; start < this.LastLnum; start++ {
		fmt.Fprintln(w, buffer[start%nlines])
	}
	return
}

func Follow(r io.Reader, w io.Writer, nlines int, follow bool) {
	tail := NewTail(r)
	tail.Follow = follow
	for !tail.Run(w, nlines) {

	}
	return
}

var option_nlines = flag.Int("n", 10, "output the last NUM lines, instead of the last 10")

func Main() error {
	args := flag.Args()
	if len(args) > 0 {
		for _, fname := range args {
			fd, err := os.Open(fname)
			if err != nil {
				return err
			}
			Follow(fd, os.Stdout, *option_nlines, true)
			fd.Close()
		}
	} else {
		Follow(os.Stdin, os.Stdout, *option_nlines, true)
	}
	return nil
}

func main() {
	flag.Parse()
	if err := Main(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
