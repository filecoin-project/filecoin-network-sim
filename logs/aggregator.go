package logs

import (
	"bufio"
	"io"
)

// LineAggregator mixes in multile readers into one,
// line by line, ensuring interleaved writes dont break up
// lines. Uses goroutines to read from and write out.
type LineAggregator struct {
	pr *io.PipeReader
	pw *io.PipeWriter
}

func NewLineAggregator() *LineAggregator {
	pr, pw := io.Pipe()
	return &LineAggregator{pr, pw}
}

func (a *LineAggregator) Reader() io.Reader {
	return a.pr
}

func (a *LineAggregator) Close() error {
	return a.pw.CloseWithError(io.EOF)
}

func (a *LineAggregator) MixReader(r io.Reader) {
	go a.mixScanner(bufio.NewReader(r))
}

func (a *LineAggregator) mixScanner(s *bufio.Reader) {
	for {
		l, err := s.ReadBytes('\n')
		if err != nil {
			return // bail out.
		}

		// pipe gates sequentially.
		// without pipe, may need a lock to ensure no interleaving.
		_, err = a.pw.Write(l)
		if err != nil {
			return // bail out.
		}
	}
}
