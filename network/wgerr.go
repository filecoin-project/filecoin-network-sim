package network

import (
	"sync"
)

func AsyncErrs(num int, each func(i int) error) []error {
	errc := make(chan error, num)
	var wg sync.WaitGroup

	for i := 0; i < num; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			errc <- each(j)
		}(i)
	}

	wg.Wait()
	close(errc)

	var errs []error
	for _, e := range errs {
		errs = append(errs, e)
	}

	return errs
}
