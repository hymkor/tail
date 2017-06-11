package main

import (
	"bufio"
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
	ticker := time.NewTicker(time.Second)
	start := this.LastLnum
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
				goto exit
			}
		case <-ticker.C:
			if stop == this.LastLnum {
				// no lines read since last timeout
				eof = false
				goto exit
			} else {
				// refresh Timeout
				stop = this.LastLnum
				break
			}
		}
	}
exit:
	ticker.Stop()
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

func SimpleTail(r io.Reader, w io.Writer, nlines int) {
	lines := make([]string, nlines)
	i := 0
	scnr := bufio.NewScanner(r)
	for scnr.Scan() {
		lines[i%nlines] = scnr.Text()
		i++
	}
	start := 0
	if i > nlines {
		start = i - nlines
	}
	for j := start; j < i; j++ {
		fmt.Fprintln(w, lines[j%nlines])
	}

}

var option_nlines = flag.Int("n", 10, "output the last NUM lines, instead of the last 10")
var option_sleep_interval = flag.Int("s", 1, "sleep for approximately N seconds(default 1.0)")

var option_follow = flag.Bool("f", false, "output appended data as the file grows")

func Main() error {
	d := time.Duration(*option_sleep_interval) * time.Second
	args := flag.Args()
	if len(args) > 0 {
		if *option_follow {
			return Watch(args[0], os.Stdout, d)
		} else {
			fd, err := os.Open(args[0])
			if err != nil {
				return err
			}
			SimpleTail(fd, os.Stdout, *option_nlines)
			fd.Close()
		}
	} else {
		if *option_follow {
			Follow(os.Stdin, os.Stdout, *option_nlines, d)
		} else {
			SimpleTail(os.Stdin, os.Stdout, *option_nlines)
		}
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
