package retries

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func ExampleRetries_Do() {
	retry := Retries{
		Delay:    time.Second,
		MaxCount: 3,
	}
	err := retry.Do(context.Background(), func() error {
		fmt.Println("hello world")
		return errors.New("test")
	})
	fmt.Println(err)

	// Output:
	// hello world
	// hello world
	// hello world
	// reach max count
}
