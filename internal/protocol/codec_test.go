package protocol

import (
	"bytes"
	"io"
	"testing"
)

func TestLengthPrefixCodecDecodesBackToBackFrames(t *testing.T) {
	codec := NewLengthPrefixCodec()
	var stream bytes.Buffer

	if _, err := codec.Encode(&stream, NewChatMessage("alice", "hello")); err != nil {
		t.Fatalf("encode first message: %v", err)
	}
	if _, err := codec.Encode(&stream, NewChatMessage("bob", "hi")); err != nil {
		t.Fatalf("encode second message: %v", err)
	}

	first, err := codec.Decode(&stream)
	if err != nil {
		t.Fatalf("decode first message: %v", err)
	}
	second, err := codec.Decode(&stream)
	if err != nil {
		t.Fatalf("decode second message: %v", err)
	}

	if first.From != "alice" || first.Content != "hello" {
		t.Fatalf("unexpected first message: %#v", first)
	}
	if second.From != "bob" || second.Content != "hi" {
		t.Fatalf("unexpected second message: %#v", second)
	}
}

func TestLengthPrefixCodecDecodesFragmentedFrame(t *testing.T) {
	codec := NewLengthPrefixCodec()
	var frame bytes.Buffer

	if _, err := codec.Encode(&frame, NewPrivateMessage("alice", "bob", "secret")); err != nil {
		t.Fatalf("encode message: %v", err)
	}

	msg, err := codec.Decode(oneByteReader{r: bytes.NewReader(frame.Bytes())})
	if err != nil {
		t.Fatalf("decode fragmented message: %v", err)
	}

	if msg.Type != TypePrivate || msg.From != "alice" || msg.To != "bob" || msg.Content != "secret" {
		t.Fatalf("unexpected message: %#v", msg)
	}
}

type oneByteReader struct {
	r io.Reader
}

func (r oneByteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.r.Read(p)
}
