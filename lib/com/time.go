package com

import "time"

func Ms2Duration(ms int) time.Duration {
	return time.Duration(ms) * time.Millisecond
}

func Sec2Duration(s int) time.Duration {
	return time.Duration(s) * time.Second
}

func Minute2Duration(m int) time.Duration {
	return time.Duration(m) * time.Minute
}

func Hour2Duration(h int) time.Duration {
	return time.Duration(h) * time.Hour
}

func Day2Duration(d int) time.Duration {
	return time.Duration(d) * time.Hour * 24
}
