package fastcdc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"
)

func newSha256(path string) func(t *testing.T) []byte {
	file, err := os.Open(path)
	if err == nil {
		defer file.Close()
		defer file.Seek(0, 0)
		hasher := sha256.New()
		if _, err = io.Copy(hasher, file); err == nil {
			return func(t *testing.T) []byte {
				t.Helper()
				return hasher.Sum(nil)
			}
		}
	}
	return func(t *testing.T) []byte {
		t.Helper()
		t.Fatal(err)
		return nil
	}
}

var sekienSha256 = newSha256("fixtures/SekienAkashita.jpg")

func handleError(err error) {
	if err != nil {
		panic(err)
	}
}

// In this example, the chunker is configured to output chunk of an average of 32kb size.
func Example_basic() {
	file, err := os.Open("fixtures/SekienAkashita.jpg")
	handleError(err)
	defer file.Close()

	chunker, err := NewChunker(context.Background(), With32kChunks())
	handleError(err)

	err = chunker.Split(file, func(offset, length uint, chunk []byte) error {
		// the chunk is only valid in the callback, copy it for later use
		fmt.Printf("offset: %d, length: %d, sum: %x\n", offset, length, sha256.Sum256(chunk))
		return nil
	})
	handleError(err)

	err = chunker.Finalize(func(offset, length uint, chunk []byte) error {
		// the chunk is only valid in the callback, copy it for later use
		fmt.Printf("offset: %d, length: %d, sum: %x\n", offset, length, sha256.Sum256(chunk))
		return nil
	})
	handleError(err)
	// Output:
	// offset: 0, length: 32857, sum: 5a80871bad4588c7278d39707fe68b8b174b1aa54c59169d3c2c72f1e16ef46d
	// offset: 32857, length: 16408, sum: 13f6a4c6d42df2b76c138c13e86e1379c203445055c2b5f043a5f6c291fa520d
	// offset: 49265, length: 60201, sum: 0fe7305ba21a5a5ca9f89962c5a6f3e29cd3e2b36f00e565858e0012e5f8df36
}

// In this example, the chunker is configured in stream mode to split a file stream part by part into chunk of an average of 32kb.
// For the sake of simplicity, the stream is simulated by cutting the file in multiple parts before the split.
func Example_stream() {
	file, err := os.Open("fixtures/SekienAkashita.jpg")
	handleError(err)
	defer file.Close()

	chunker, err := NewChunker(context.Background(), WithStreamMode(), With32kChunks())
	handleError(err)

	buf := make([]byte, 3*65_536)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			handleError(err)
		}
		err = chunker.Split(bytes.NewReader(buf[:n]), func(offset, length uint, chunk []byte) error {
			// the chunk is only valid in the callback, copy it for later use
			fmt.Printf("offset: %d, length: %d, sum: %x\n", offset, length, sha256.Sum256(chunk))
			return nil
		})
		handleError(err)
	}

	err = chunker.Finalize(func(offset, length uint, chunk []byte) error {
		// the chunk is only valid in the callback, copy it for later use
		fmt.Printf("offset: %d, length: %d, sum: %x\n", offset, length, sha256.Sum256(chunk))
		return nil
	})
	handleError(err)
	// Output:
	// offset: 0, length: 32857, sum: 5a80871bad4588c7278d39707fe68b8b174b1aa54c59169d3c2c72f1e16ef46d
	// offset: 32857, length: 16408, sum: 13f6a4c6d42df2b76c138c13e86e1379c203445055c2b5f043a5f6c291fa520d
	// offset: 49265, length: 60201, sum: 0fe7305ba21a5a5ca9f89962c5a6f3e29cd3e2b36f00e565858e0012e5f8df36
}

func TestLogarithm2(t *testing.T) {
	tests := []struct {
		Value, Result uint
	}{
		{65537, 16},
		{65536, 16},
		{65535, 16},
		{32769, 15},
		{32768, 15},
		{32767, 15},
		{AverageMin, 8},
		{AverageMax, 28},
	}

	for _, tc := range tests {
		got := logarithm2(tc.Value)
		if got != tc.Result {
			t.Errorf("want = %d, got = %d", tc.Result, got)
		}
	}
}

func TestCeilDiv(t *testing.T) {
	tests := []struct {
		X, Y, Result uint
	}{
		{10, 5, 2},
		{11, 5, 3},
		{10, 3, 4},
		{9, 3, 3},
		{6, 2, 3},
		{5, 2, 3},
		{1, 2, 1},
	}

	for _, tc := range tests {
		got := ceilDiv(tc.X, tc.Y)
		if got != tc.Result {
			t.Errorf("want = %d, got = %d", tc.Result, got)
		}
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		Point, Carry, Min, Result uint
	}{
		{500, 300, 1, 200},
		{200, 500, 1, 1},
		{2, 1, 1, 1},
		{1, 2, 1, 1},
		{1, 1, 1, 1},
	}

	for _, tc := range tests {
		got := min(tc.Point, tc.Carry, tc.Min)
		if got != tc.Result {
			t.Errorf("want = %d, got = %d", tc.Result, got)
		}
	}
}

func TestCenterSize(t *testing.T) {
	tests := []struct {
		Average, Min, SourceSize, Result uint
	}{
		{50, 100, 50, 0},
		{200, 100, 50, 50},
		{200, 100, 40, 40},
	}

	for _, tc := range tests {
		got := centerSize(tc.Average, tc.Min, tc.SourceSize)
		if got != tc.Result {
			t.Errorf("want = %d, got = %d", tc.Result, got)
		}
	}
}

func TestMask(t *testing.T) {
	tests := []struct {
		Bits, Result uint
	}{
		{24, 16_777_215},
		{16, 65535},
		{10, 1023},
		{8, 255},
	}

	for _, tc := range tests {
		got := mask(tc.Bits)
		if got != tc.Result {
			t.Errorf("want = %d, got = %d", tc.Result, got)
		}
	}
}

func TestMaskPanic(t *testing.T) {
	tests := []struct {
		Name     string
		Bits     uint
		PanicMsg string
	}{
		{"too low", 0, "bits too low"},
		{"too high", 32, "bits too high"},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("the code did not panic")
				} else {
					panicMsg := r.(string)
					if panicMsg != tc.PanicMsg {
						t.Errorf("want = %s, got = %s", tc.PanicMsg, r)
					}
				}
			}()
			mask(tc.Bits)
		})
	}
}

func TestSplitPanic(t *testing.T) {
	size := 32 * 1024 * 1024
	data := randomData(155, size)

	defer func() {
		if r := recover(); r == nil {
			t.Error("the code did not panic")
		} else {
			panicMsg := r.(string)
			want := "split must not be call multiple time in regular mode, use stream mode instead"
			if panicMsg != want {
				t.Errorf("want = %s, got = %s", want, r)
			}
		}
	}()

	chunker, err := NewChunker(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = chunker.Split(bytes.NewReader(data), func(offset, length uint, chunk []byte) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = chunker.Split(bytes.NewReader(data), func(offset, length uint, chunk []byte) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = chunker.Finalize(func(offset, length uint, chunk []byte) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFinalizePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("the code did not panic")
		} else {
			panicMsg := r.(string)
			want := "finalize most succeed a split, call split first"
			if panicMsg != want {
				t.Errorf("want = %s, got = %s", want, r)
			}
		}
	}()

	chunker, err := NewChunker(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = chunker.Finalize(func(offset, length uint, chunk []byte) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestChunkerValidation(t *testing.T) {
	tests := map[string]struct {
		MinSize, AvgSize, MaxSize, BufferSize uint
		Want                                  error
	}{
		"minimum min size": {
			MinSize:    MinimumMin - 1,
			AvgSize:    AverageMin,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunksSizePoint,
		},
		"average min size": {
			MinSize:    MinimumMin,
			AvgSize:    AverageMin - 1,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunksSizePoint,
		},
		"maximum min size": {
			MinSize:    MinimumMin,
			AvgSize:    AverageMin,
			MaxSize:    MaximumMin - 1,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunksSizePoint,
		},
		"minimum max size": {
			MinSize:    MinimumMax + 1,
			AvgSize:    AverageMax,
			MaxSize:    MaximumMax,
			BufferSize: MaximumMax,
			Want:       ErrInvalidChunksSizePoint,
		},
		"average max size": {
			MinSize:    MinimumMax,
			AvgSize:    AverageMax + 1,
			MaxSize:    MaximumMax,
			BufferSize: MaximumMax,
			Want:       ErrInvalidChunksSizePoint,
		},
		"maximum max size": {
			MinSize:    MinimumMax,
			AvgSize:    AverageMax,
			MaxSize:    MaximumMax + 1,
			BufferSize: MaximumMax,
			Want:       ErrInvalidChunksSizePoint,
		},
		"minimum buffer length": {
			MinSize:    MinimumMin,
			AvgSize:    AverageMin,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin - 1,
			Want:       ErrInvalidBufferLength,
		},
		"min size bigger or equal than avg size": {
			MinSize:    AverageMin,
			AvgSize:    AverageMin,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunksSizePoint,
		},
		"max size smaller or equal than avg size": {
			MinSize:    MinimumMin,
			AvgSize:    MaximumMin,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunksSizePoint,
		},
		"proportional cut point": {
			MinSize:    1048,
			AvgSize:    2048,
			MaxSize:    3096,
			BufferSize: 2 * 3096,
			Want:       ErrInvalidChunksSizePoint,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewChunker(context.Background(), WithChunksSize(tc.MinSize, tc.AvgSize, tc.MaxSize), WithBufferSize(tc.BufferSize))
			if !errors.Is(err, tc.Want) {
				t.Errorf("want = %s, got = %s", tc.Want, err)
			}
		})
	}
}

func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	size := 32 * 1024 * 1024
	data := randomData(155, size)
	chunker, err := NewChunker(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = chunker.Split(bytes.NewReader(data), func(offset, length uint, chunk []byte) error {
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want = %s, got = %s", context.Canceled, err)
	}

	err = chunker.Finalize(func(offset, length uint, chunk []byte) error {
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want = %s, got = %s", context.Canceled, err)
	}
}

func TestAllZeros(t *testing.T) {
	type Chunk struct {
		Offset uint
		Length uint
	}
	buffer := make([]byte, 10240)
	chunker, err := NewChunker(context.Background(), WithChunksSize(64, 256, 1024), WithBufferSize(1024))
	if err != nil {
		t.Fatal(err)
	}

	chunks := make([]Chunk, 0, 6)
	if err := chunker.Split(bytes.NewBuffer(buffer), func(offset, length uint, chunk []byte) error {
		chunks = append(chunks, Chunk{offset, length})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
		chunks = append(chunks, Chunk{offset, length})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, chunk := range chunks {
		if chunk.Offset%1024 != 0 {
			t.Errorf("offset: want = 0, got = %d", chunk.Offset%1024)
		}
		if chunk.Length != 1024 {
			t.Errorf("length: want = 1024, got = %d", chunk.Length)
		}
	}
}

func TestRandomInputFuzz(t *testing.T) {
	tests := []struct {
		Name    string
		MinSize int
		MaxSize int
		Opt     Option
	}{
		{"16kChunks", 8192, 32768, With16kChunks()},
		{"32kChunks", 16384, 65_536, With32kChunks()},
		{"64kChunks", 32_768, 131_072, With64kChunks()},
	}

	seed := time.Now().UnixNano()
	rand.Seed(seed)
	t.Logf("seed, %d", seed)

	type Chunk struct {
		Offset uint
		Length uint
		Chunk  []byte
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			max := 1 * 1024 * 1024  // max buffer size
			min := tc.MaxSize       // min buffer size for the chunk size range, it's set to the max chunks size
			sMax := 1 * 1024 * 1024 // max stream buffer size
			sMin := 1000            // min stream buffer size

			// repeat test
			for i := 0; i < 5000; i++ {
				rd := rand.Intn(8*1024*1024-1000+1) + 1000
				data := make([]byte, rd)
				rand.Read(data)
				file := bytes.NewReader(data)

				hasher := sha256.New()
				io.Copy(hasher, file)
				sum := hasher.Sum(nil)
				file.Seek(0, 0)

				bufSize := uint(rand.Intn(max-min+1) + min)
				sBufSize := uint(rand.Intn(sMax-sMin+1) + sMin)

				chunks := make([]Chunk, 0)
				chunker, err := NewChunker(context.Background(), tc.Opt, WithBufferSize(bufSize))
				if err != nil {
					t.Fatal(err)
				}

				regularHasher := sha256.New()
				if err := chunker.Split(file, func(offset, length uint, chunk []byte) error {
					chunks = append(chunks, Chunk{offset, length, chunk})
					io.Copy(regularHasher, bytes.NewReader(chunk))
					return nil
				}); err != nil {
					t.Fatal(err)
				}

				if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
					chunks = append(chunks, Chunk{offset, length, chunk})
					io.Copy(regularHasher, bytes.NewReader(chunk))
					return nil
				}); err != nil {
					t.Fatal(err)
				}

				file.Seek(0, 0)

				chunker, err = NewChunker(context.Background(), WithStreamMode(), tc.Opt, WithBufferSize(bufSize))
				if err != nil {
					t.Fatal(err)
				}

				streamHasher := sha256.New()
				chunksStream := make([]Chunk, 0)
				buf := make([]byte, sBufSize)
				for {
					n, err := file.Read(buf)
					if err != nil {
						if err == io.EOF {
							break
						}
						t.Fatal(err)
					}

					if err := chunker.Split(bytes.NewReader(buf[:n]), func(offset, length uint, chunk []byte) error {
						chunksStream = append(chunksStream, Chunk{offset, length, chunk})
						io.Copy(streamHasher, bytes.NewReader(chunk))
						return nil
					}); err != nil {
						t.Fatal(err)
					}

				}
				if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
					chunksStream = append(chunksStream, Chunk{offset, length, chunk})
					io.Copy(streamHasher, bytes.NewReader(chunk))
					return nil
				}); err != nil {
					t.Fatal(err)
				}

				if len(chunks) != len(chunksStream) {
					t.Errorf("length: want = %d, got = %d, buffer length = %d, stream buffer length = %d, file size = %d", len(chunks), len(chunksStream), bufSize, sBufSize, rd)
					file.Seek(0, 0)
					continue
				}

				for i, chunk := range chunks {
					if chunk.Offset != chunksStream[i].Offset {
						t.Errorf("offset: want = %d, got = %d, buffer length = %d, stream buffer length = %d, file size = %d", chunk.Offset, chunksStream[i].Offset, bufSize, sBufSize, rd)
					}
					if chunk.Length != chunksStream[i].Length {
						t.Errorf("length: want = %d, got = %d, buffer length = %d, stream buffer length = %d, file size = %d", chunk.Offset, chunksStream[i].Offset, bufSize, sBufSize, rd)
					}
					if chunk.Length != uint(len(chunk.Chunk)) {
						t.Errorf("regular split: length mismatch: want = %d, got = %d, buffer length = %d, file size = %d", chunk.Length, uint(len(chunk.Chunk)), bufSize, rd)
					}
					if chunksStream[i].Length != uint(len(chunksStream[i].Chunk)) {
						t.Errorf("stream split: length mismatch: want = %d, got = %d, buffer length = %d, stream buffer length = %d, file size = %d", chunk.Length, uint(len(chunk.Chunk)), bufSize, sBufSize, rd)
					}
					if (chunk.Length < uint(tc.MinSize) || chunk.Length > uint(tc.MaxSize)) && i != len(chunks)-1 {
						t.Errorf("regular split: chunks size: %d < %d < %d, buffer length = %d, file size = %d", tc.MinSize, chunk.Length, tc.MaxSize, bufSize, rd)
					}
					if (chunksStream[i].Length < uint(tc.MinSize) || chunksStream[i].Length > uint(tc.MaxSize)) && i != len(chunksStream)-1 {
						t.Errorf("regular split: chunks size: %d < %d < %d, buffer length = %d, stream buffer length = %d, file size = %d", tc.MinSize, chunksStream[i].Length, tc.MaxSize, bufSize, sBufSize, rd)
					}
				}

				regularSum := regularHasher.Sum(nil)
				if !reflect.DeepEqual(sum, regularSum) {
					t.Errorf("regular chunking: sum mismatch: want = %x, got = %x, buffer = %d, , file size = %d", sum, regularSum, bufSize, rd)
				}
				streamSum := streamHasher.Sum(nil)
				if !reflect.DeepEqual(sum, streamSum) {
					t.Errorf("stream chunking: sum mismatch: want = %x, got = %x, buffer = %d, stream buffer = %d, file size = %d", sum, streamSum, bufSize, sBufSize, rd)
				}

				file.Seek(0, 0)
			}
		})
	}
}

func TestSekienFuzz(t *testing.T) {
	type Chunk struct {
		Offset uint
		Length uint
	}

	tests := []struct {
		Name    string
		MinSize int
		MaxSize int
		Opt     Option
		Want    []Chunk
	}{
		{
			Name:    "16kChunks",
			MinSize: 8192,
			MaxSize: 32768,
			Opt:     With16kChunks(),
			Want: []Chunk{
				{0, 22366},
				{22366, 8282},
				{30648, 16303},
				{46951, 18696},
				{65647, 32768},
				{98415, 11051},
			},
		},
		{
			Name:    "32kChunks",
			MinSize: 16384,
			MaxSize: 65_536,
			Opt:     With32kChunks(),
			Want: []Chunk{
				{0, 32857},
				{32857, 16408},
				{49265, 60201},
			},
		},
		{
			Name:    "64kChunks",
			MinSize: 32_768,
			MaxSize: 131_072,
			Opt:     With64kChunks(),
			Want: []Chunk{
				{0, 32857},
				{32857, 76609},
			},
		},
	}

	file, err := os.Open("fixtures/SekienAkashita.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	rand.Seed(time.Now().UnixNano())
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			max := 1 * 1024 * 1024  // max buffer size
			min := tc.MaxSize       // min buffer size for the chunk size range, it's set to the max chunks size
			sMax := 1 * 1024 * 1024 // max stream buffer size
			sMin := 1000            // min stream buffer size

			// repeat test
			for i := 0; i < 10000; i++ {
				bufSize := uint(rand.Intn(max-min+1) + min)
				sBufSize := uint(rand.Intn(sMax-sMin+1) + sMin)

				type Chunk struct {
					Offset uint
					Length uint
					Chunk  []byte
				}

				chunks := make([]Chunk, 0)
				chunker, err := NewChunker(context.Background(), tc.Opt, WithBufferSize(bufSize))
				if err != nil {
					t.Fatal(err)
				}

				regularHasher := sha256.New()
				if err := chunker.Split(file, func(offset, length uint, chunk []byte) error {
					chunks = append(chunks, Chunk{offset, length, chunk})
					io.Copy(regularHasher, bytes.NewReader(chunk))
					return nil
				}); err != nil {
					t.Fatal(err)
				}

				if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
					io.Copy(regularHasher, bytes.NewReader(chunk))
					chunks = append(chunks, Chunk{offset, length, chunk})
					return nil
				}); err != nil {
					t.Fatal(err)
				}

				file.Seek(0, 0)

				chunker, err = NewChunker(context.Background(), WithStreamMode(), tc.Opt, WithBufferSize(bufSize))
				if err != nil {
					t.Fatal(err)
				}

				streamHasher := sha256.New()
				chunksStream := make([]Chunk, 0)
				buf := make([]byte, sBufSize)
				for {
					n, err := file.Read(buf)
					if err != nil {
						if err == io.EOF {
							break
						}
						t.Fatal(err)
					}
					if err := chunker.Split(bytes.NewReader(buf[:n]), func(offset, length uint, chunk []byte) error {
						chunksStream = append(chunksStream, Chunk{offset, length, chunk})
						io.Copy(streamHasher, bytes.NewReader(chunk))
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				}
				if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
					io.Copy(streamHasher, bytes.NewReader(chunk))
					chunksStream = append(chunksStream, Chunk{offset, length, chunk})
					return nil
				}); err != nil {
					t.Fatal(err)
				}

				if len(chunks) != len(tc.Want) {
					t.Errorf("length: want = %d, got = %d, buffer length = %d, stream buffer length = %d", len(chunks), len(tc.Want), bufSize, sBufSize)
					file.Seek(0, 0)
					continue
				}

				if len(chunksStream) != len(tc.Want) {
					t.Errorf("length: want = %d, got = %d, buffer length = %d, stream buffer length = %d", len(chunksStream), len(tc.Want), bufSize, sBufSize)
					file.Seek(0, 0)
					continue
				}

				for i, chunk := range chunks {
					if chunk.Offset != tc.Want[i].Offset {
						t.Errorf("offset: want = %d, got = %d, buffer = %d, stream buffer length = %d", chunk.Offset, tc.Want[i].Offset, bufSize, sBufSize)
					}
					if chunk.Length != tc.Want[i].Length {
						t.Errorf("length: want = %d, got = %d, buffer = %d, stream buffer length = %d", chunk.Offset, tc.Want[i].Offset, bufSize, sBufSize)
					}
					if chunk.Length != uint(len(chunk.Chunk)) {
						t.Errorf("regular split: length mismatch: want = %d, got = %d, buffer length = %d", chunk.Length, uint(len(chunk.Chunk)), bufSize)
					}
				}

				for i, chunk := range chunksStream {
					if chunk.Offset != tc.Want[i].Offset {
						t.Errorf("offset: want = %d, got = %d, buffer = %d, stream buffer length = %d", chunk.Offset, tc.Want[i].Offset, bufSize, sBufSize)
					}
					if chunk.Length != tc.Want[i].Length {
						t.Errorf("length: want = %d, got = %d, buffer = %d, stream buffer length = %d", chunk.Offset, tc.Want[i].Offset, bufSize, sBufSize)
					}
					if chunk.Length != uint(len(chunk.Chunk)) {
						t.Errorf("regular split: length mismatch: want = %d, got = %d, buffer length = %d", chunk.Length, uint(len(chunk.Chunk)), bufSize)
					}
				}

				regularSum := regularHasher.Sum(nil)
				if !reflect.DeepEqual(sekienSha256(t), regularSum) {
					t.Errorf("regular chunking: sum mismatch: want = %x, got = %x, buffer = %d", sekienSha256(t), regularSum, bufSize)
				}
				streamSum := streamHasher.Sum(nil)
				if !reflect.DeepEqual(sekienSha256(t), streamSum) {
					t.Errorf("stream chunking: sum mismatch: want = %x, got = %x, buffer = %d, stream buffer = %d", sekienSha256(t), streamSum, bufSize, sBufSize)
				}

				file.Seek(0, 0)
			}
		})
	}
}

func TestSekienChunks(t *testing.T) {
	file, err := os.Open("fixtures/SekienAkashita.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	type Chunk struct {
		Offset uint
		Length uint
	}

	cases := map[string]struct {
		Preset     Option
		BufferSize uint
		Want       []Chunk
	}{
		"16kChunks": {
			Preset: With16kChunks(),
			Want: []Chunk{
				{0, 22366},
				{22366, 8282},
				{30648, 16303},
				{46951, 18696},
				{65647, 32768},
				{98415, 11051},
			},
			BufferSize: 32768,
		},
		"32kChunks": {
			Preset: With32kChunks(),
			Want: []Chunk{
				{0, 32857},
				{32857, 16408},
				{49265, 60201},
			},
			BufferSize: 65_536,
		},
		"64kChunks": {
			Preset: With64kChunks(),
			Want: []Chunk{
				{0, 32857},
				{32857, 76609},
			},
			BufferSize: 131_072,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			defer file.Seek(0, 0)

			chunks := make([]Chunk, 0, 6)
			chunker, err := NewChunker(context.Background(), tc.Preset, WithBufferSize(tc.BufferSize))
			if err != nil {
				t.Fatal(err)
			}

			hasher := sha256.New()
			if err := chunker.Split(file, func(offset, length uint, chunk []byte) error {
				chunks = append(chunks, Chunk{offset, length})
				_, err := io.Copy(hasher, bytes.NewReader(chunk))
				return err
			}); err != nil {
				t.Fatal(err)
			}

			if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
				chunks = append(chunks, Chunk{offset, length})
				_, err := io.Copy(hasher, bytes.NewReader(chunk))
				return err
			}); err != nil {
				t.Fatal(err)
			}

			if len(chunks) != len(tc.Want) {
				t.Fatalf("chunks length: want = %d, got = %d", len(tc.Want), len(chunks))
			}
			for i, res := range tc.Want {
				if chunks[i].Offset != res.Offset || chunks[i].Length != res.Length {
					t.Errorf("chunks[%d] : want offset = %d, got offset = %d, want length = %d, got length = %d", i, res.Offset, chunks[i].Offset, res.Length, chunks[i].Length)
				}
			}

			sum := hasher.Sum(nil)
			if !reflect.DeepEqual(sekienSha256(t), sum) {
				t.Errorf("sum mismatch: want = %x, got = %x", sekienSha256(t), sum)
			}
		})
	}
}

func TestSekienChunksStream(t *testing.T) {
	file, err := os.Open("fixtures/SekienAkashita.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	type Chunk struct {
		Offset uint
		Length uint
	}

	cases := map[string]struct {
		Preset     Option
		BufferSize uint
		Want       []Chunk
	}{
		"16kChunks": {
			Preset: With16kChunks(),
			Want: []Chunk{
				{0, 22366},
				{22366, 8282},
				{30648, 16303},
				{46951, 18696},
				{65647, 32768},
				{98415, 11051},
			},
			BufferSize: 32768,
		},
		"32kChunks": {
			Preset: With32kChunks(),
			Want: []Chunk{
				{0, 32857},
				{32857, 16408},
				{49265, 60201},
			},
			BufferSize: 65_536,
		},
		"64kChunks": {
			Preset: With64kChunks(),
			Want: []Chunk{
				{0, 32857},
				{32857, 76609},
			},
			BufferSize: 131_072,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			defer file.Seek(0, 0)

			chunks := make([]Chunk, 0, 6)
			chunker, err := NewChunker(context.Background(), WithStreamMode(), tc.Preset, WithBufferSize(tc.BufferSize))
			if err != nil {
				t.Fatal(err)
			}

			hasher := sha256.New()
			buf := make([]byte, tc.BufferSize)
			for {
				n, err := file.Read(buf)
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatal(err)
				}

				if err := chunker.Split(bytes.NewReader(buf[:n]), func(offset, length uint, chunk []byte) error {
					chunks = append(chunks, Chunk{offset, length})
					_, err := io.Copy(hasher, bytes.NewReader(chunk))
					return err
				}); err != nil {
					t.Fatal(err)
				}

			}

			if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
				chunks = append(chunks, Chunk{offset, length})
				_, err := io.Copy(hasher, bytes.NewReader(chunk))
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if len(chunks) != len(tc.Want) {
				t.Fatalf("chunks length: want = %d, got = %d", len(tc.Want), len(chunks))
			}
			for i, res := range tc.Want {
				if chunks[i].Offset != res.Offset || chunks[i].Length != res.Length {
					t.Errorf("chunks[%d] : want offset = %d, got offset = %d, want length = %d, got length = %d", i, res.Offset, chunks[i].Offset, res.Length, chunks[i].Length)
				}
			}

			sum := hasher.Sum(nil)
			if !reflect.DeepEqual(sekienSha256(t), sum) {
				t.Errorf("sum mismatch: want = %x, got = %x", sekienSha256(t), sum)
			}
		})
	}
}

func TestSekien16kChunksStreamWithMissingPart(t *testing.T) {
	type Chunk struct {
		Offset uint
		Length uint
	}
	results := []Chunk{
		{0, 22366},
		{22366, 8282},
		{30648, 16303},
		{46951, 18696},
		{65647, 32768},
		{98415, 11051},
	}

	file, err := os.Open("fixtures/SekienAkashita.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	chunks := make([]Chunk, 0, 6)
	chunker, err := NewChunker(context.Background(), WithStreamMode(), With16kChunks(), WithBufferSize(32768))
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 32768)
	cpt := 0
	for {
		var n int
		var err error
		if cpt == 0 || cpt == 3 || cpt == 4 {
			n = 0
		} else {
			n, err = file.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatal(err)
			}
		}

		if err := chunker.Split(bytes.NewReader(buf[:n]), func(offset, length uint, chunk []byte) error {
			chunks = append(chunks, Chunk{offset, length})
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		cpt++
	}

	if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
		chunks = append(chunks, Chunk{offset, length})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if len(chunks) != len(results) {
		t.Fatalf("chunks length: want = %d, got = %d", len(results), len(chunks))
	}
	for i, res := range results {
		if chunks[i].Offset != res.Offset || chunks[i].Length != res.Length {
			t.Errorf("chunks[%d] : want offset = %d, got offset = %d, want length = %d, got length = %d", i, res.Offset, chunks[i].Offset, res.Length, chunks[i].Length)
		}
	}
}

func TestSekienMinChunks(t *testing.T) {
	file, err := os.Open("fixtures/SekienAkashita.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	chunker, err := NewChunker(context.Background(), WithChunksSize(64, 256, 1024), WithBufferSize(1024))
	if err != nil {
		t.Fatal(err)
	}

	hasher := sha256.New()
	if err := chunker.Split(file, func(offset, length uint, chunk []byte) error {
		_, err := io.Copy(hasher, bytes.NewReader(chunk))
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
		_, err := io.Copy(hasher, bytes.NewReader(chunk))
		return err
	}); err != nil {
		t.Fatal()
	}

	sum := hasher.Sum(nil)
	if !reflect.DeepEqual(sum, sekienSha256(t)) {
		t.Errorf("sum mismatch: want = %x, got = %x", sekienSha256(t), sum)
	}
}

func TestSekienMaxChunks(t *testing.T) {
	type Chunk struct {
		Offset uint
		Length uint
	}

	file, err := os.Open("fixtures/SekienAkashita.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	chunks := make([]Chunk, 0, 1)
	chunker, err := NewChunker(context.Background(), WithChunksSize(67_108_864, 268_435_456, 1_073_741_824), WithBufferSize(1_073_741_824))
	if err != nil {
		t.Fatal(err)
	}

	hasher := sha256.New()
	if err := chunker.Split(file, func(offset, length uint, chunk []byte) error {
		chunks = append(chunks, Chunk{offset, length})
		_, err := io.Copy(hasher, bytes.NewReader(chunk))
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
		chunks = append(chunks, Chunk{offset, length})
		_, err := io.Copy(hasher, bytes.NewReader(chunk))
		return err
	}); err != nil {
		t.Fatal()
	}

	if len(chunks) != 1 {
		t.Fatalf("chunks length: want 1, got = %d", len(chunks))
	}

	if chunks[0].Offset != 0 && chunks[0].Length != 109466 {
		t.Errorf("want offset = 0, got offset = %d, want length = 109466, got length = %d", chunks[0].Offset, chunks[0].Length)
	}

	sum := hasher.Sum(nil)
	if !reflect.DeepEqual(sum, sekienSha256(t)) {
		t.Errorf("sum mismatch: want = %x, got = %x", sekienSha256(t), sum)
	}
}

func TestSmallInput(t *testing.T) {
	dataset := make([]byte, 8193)
	rand.Seed(time.Now().UnixNano())
	rand.Read(dataset)

	chunker, err := NewChunker(context.Background(), With16kChunks())
	if err != nil {
		t.Fatal(err)
	}
	output := make([]byte, 0, 8193)
	if err := chunker.Split(bytes.NewReader(dataset), func(offset, length uint, chunk []byte) error {
		output = append(output, chunk...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
		output = append(output, chunk...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(dataset, output) {
		t.Error("chunk mismatch")
	}
}
