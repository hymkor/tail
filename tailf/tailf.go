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
}

func NewTail(r io.Reader) *Tail {
	in := make(chan string, 10)
	go reader2channel(r, in)

	return &Tail{
		Input:    in,
		LastLnum: 0,
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
			if ok { // Not Found EOF and Continue
				buffer[this.LastLnum%nlines] = line
				this.LastLnum++
				break
			} else { // Found EOF
				eof = true
				cancel()
				goto exit
			}
		case <-ctx.Done(): // Timeout
			if stop == this.LastLnum {
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
	if f, ok := w.(*os.File); ok {
		f.Sync()
	}
	return
}

func Follow(r io.Reader, w io.Writer, nlines int, d time.Duration) {
	tail := NewTail(r)
	for !tail.Run(w, nlines) {
		time.Sleep(d)
	}
	return
}

func Watch(fname string, w io.Writer, d time.Duration) error {
	var pos int64 = 0
	for {
		fd, err := os.Open(fname)
		if err != nil {
			return err
		}
		if pos > 0 {
			stat, err := fd.Stat()
			if err != nil {
				fd.Close()
				return err
			}
			if stat.Size() < pos {
				fmt.Fprintf(os.Stderr, "(%s shrinked)\n", fname)
			} else if _, err = fd.Seek(pos, 0); err != nil {
				fd.Close()
				return err
			}
		}
		Follow(fd, w, *option_nlines, d)
		pos, err = fd.Seek(0, 1)
		fd.Close()
		if err != nil {
			return err
		}
		time.Sleep(d)
	}
}

var option_nlines = flag.Int("n", 10, "output the last NUM lines, instead of the last 10")
var option_sleep_interval = flag.Int("s", 1, "sleep for approximately N seconds(default 1.0)")

func Main() error {
	d := time.Duration(*option_sleep_interval) * time.Second
	args := flag.Args()
	if len(args) > 0 {
		return Watch(args[0], os.Stdout, d)
	} else {
		Follow(os.Stdin, os.Stdout, *option_nlines, d)
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
