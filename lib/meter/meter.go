package meter

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Storage unit constants.
const (
	Byte  = 1
	KByte = Byte << 10
	MByte = KByte << 10
	GByte = MByte << 10
	TByte = GByte << 10
	PByte = TByte << 10
	EByte = PByte << 10
)

type ByteSize uint64

func (b ByteSize) Bytes() uint64 {
	return uint64(b)
}

func (b ByteSize) KBytes() float64 {
	return float64(b/KByte) + float64(b%KByte)/float64(KByte)
}

func (b ByteSize) MBytes() float64 {
	return float64(b/MByte) + float64(b%MByte)/float64(MByte)
}

func (b ByteSize) GBytes() float64 {
	return float64(b/GByte) + float64(b%GByte)/float64(GByte)
}

func (b ByteSize) TBytes() float64 {
	return float64(b/TByte) + float64(b%TByte)/float64(TByte)
}

func (b ByteSize) PBytes() float64 {
	return float64(b/PByte) + float64(b%PByte)/float64(PByte)
}

func (b ByteSize) EBytes() float64 {
	return float64(b/EByte) + float64(b%EByte)/float64(EByte)
}

func (b ByteSize) String() string {
	if b < 10 {
		return fmt.Sprintf("%dB", b)
	}
	sizes := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	e := math.Floor(math.Log(float64(b)) / math.Log(1024))
	val := float64(b) / math.Pow(1024, e)
	return fmt.Sprintf("%0.1f%s", val, sizes[int(e)])
}

func (b ByteSize) HumanSize() string {
	return b.String()
}

func (b ByteSize) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

func (b *ByteSize) UnmarshalText(t []byte) error {
	var val uint64
	var c byte
	var i int
	var cutoff uint64 = math.MaxUint64 / 10

	// backup for error message
	t0 := t

loop:
	for i < len(t) {
		c = t[i]
		switch {
		case '0' <= c && c <= '9':
			if val > cutoff {
				return &strconv.NumError{
					Func: "UnmarshalText",
					Num:  string(t0),
					Err:  strconv.ErrRange,
				}
			}

			c = c - '0'
			val *= 10

			if val > val+uint64(c) { // val+v overflows
				return &strconv.NumError{
					Func: "UnmarshalText",
					Num:  string(t0),
					Err:  strconv.ErrRange,
				}
			}
			val += uint64(c)
			i++

		default:
			if i == 0 {
				*b = 0
				return &strconv.NumError{
					Func: "UnmarshalText",
					Num:  string(t0),
					Err:  strconv.ErrSyntax,
				}
			}
			break loop
		}
	}

	unit := uint64(Byte)
	unitStr := strings.ToLower(strings.TrimSpace(string(t[i:])))
	switch unitStr {
	case "", "b", "byte": // do nothing
	case "k", "kb", "kilo", "kilobyte", "kilobytes":
		unit = KByte
	case "m", "mb", "mega", "megabyte", "megabytes":
		unit = MByte
	case "g", "gb", "giga", "gigabyte", "gigabytes":
		unit = GByte
	case "t", "tb", "tera", "terabyte", "terabytes":
		unit = TByte
	case "p", "pb", "peta", "petabyte", "petabytes":
		unit = PByte
	case "e", "ebyte", "eb":
		unit = EByte
	default:
		*b = 0
		return &strconv.NumError{
			Func: "UnmarshalText",
			Num:  string(t0),
			Err:  strconv.ErrSyntax,
		}
	}
	if val > math.MaxUint64/unit {
		*b = ByteSize(math.MaxUint64)
		return &strconv.NumError{
			Func: "UnmarshalText",
			Num:  string(t0),
			Err:  strconv.ErrRange,
		}
	}
	*b = ByteSize(val * unit)
	return nil
}

func ParseBytes(s string) (uint64, error) {
	v := ByteSize(0)
	err := v.UnmarshalText([]byte(s))
	return v.Bytes(), err
}

func HumanSize(bytes uint64) (s string) {
	return ByteSize(bytes).HumanSize()
}
