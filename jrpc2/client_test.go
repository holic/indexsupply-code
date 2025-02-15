package jrpc2

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/indexsupply/x/eth"
	"github.com/indexsupply/x/shovel/glf"
	"kr.dev/diff"
)

var (
	//go:embed testdata/block-18000000.json
	block18000000JSON string
	//go:embed testdata/logs-18000000.json
	logs18000000JSON string

	//go:embed testdata/block-1000001.json
	block1000001JSON string
	//go:embed testdata/logs-1000001.json
	logs1000001JSON string
)

func TestLatest_Cached(t *testing.T) {
	var counter int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		diff.Test(t, t.Fatalf, nil, err)
		switch {
		case strings.Contains(string(body), "eth_getBlockByNumber"):
			switch counter {
			case 0:
				_, err := w.Write([]byte(`{"result": {
					"hash": "0x95b198e154acbfc64109dfd22d8224fe927fd8dfdedfae01587674482ba4baf3",
					"number": "0x112a880"
				}}`))
				diff.Test(t, t.Fatalf, nil, err)
			case 1:
				_, err := w.Write([]byte(`{"result": {
					"hash": "0xd5ca78be6c6b42cf929074f502cef676372c26f8d0ba389b6f9b5d612d70f815",
					"number": "0x112a881"
				}}`))
				diff.Test(t, t.Fatalf, nil, err)
			}
		}
		counter++
	}))
	defer ts.Close()
	var (
		c         = New(ts.URL)
		n, h, err = c.Latest(0)
	)
	diff.Test(t, t.Errorf, nil, err)
	diff.Test(t, t.Errorf, counter, 1)
	diff.Test(t, t.Errorf, n, uint64(18000000))
	diff.Test(t, t.Errorf, eth.EncodeHex(h), "0x95b198e154acbfc64109dfd22d8224fe927fd8dfdedfae01587674482ba4baf3")

	_, _, err = c.Latest(18000000 - 1)
	diff.Test(t, t.Errorf, nil, err)
	diff.Test(t, t.Errorf, counter, 1)

	n, h, err = c.Latest(18000000)
	diff.Test(t, t.Errorf, nil, err)
	diff.Test(t, t.Errorf, counter, 2)
	diff.Test(t, t.Errorf, n, uint64(18000001))
	diff.Test(t, t.Errorf, eth.EncodeHex(h), "0xd5ca78be6c6b42cf929074f502cef676372c26f8d0ba389b6f9b5d612d70f815")

	_, _, err = c.Latest(18000000)
	diff.Test(t, t.Errorf, nil, err)
	diff.Test(t, t.Errorf, counter, 2)
}

func TestError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		diff.Test(t, t.Fatalf, nil, err)
		switch {
		case strings.Contains(string(body), "eth_getBlockByNumber"):
			_, err := w.Write([]byte(`
				[{
					"jsonrpc": "2.0",
					"id": "1",
					"error": {"code": -32012, "message": "credits"}
				}]
			`))
			diff.Test(t, t.Fatalf, nil, err)
		}
	}))
	defer ts.Close()

	var (
		c      = New(ts.URL)
		want   = "getting blocks: cache get: rpc=eth_getBlockByNumber code=-32012 msg=credits"
		_, got = c.Get(&glf.Filter{UseBlocks: true}, 1000001, 1)
	)
	diff.Test(t, t.Errorf, want, got.Error())
}

func TestGet(t *testing.T) {
	const start, limit = 10, 5
	blocks, err := New("").Get(&glf.Filter{}, start, limit)
	diff.Test(t, t.Fatalf, nil, err)
	diff.Test(t, t.Fatalf, len(blocks), limit)
	diff.Test(t, t.Fatalf, blocks[0].Num(), uint64(10))
	diff.Test(t, t.Fatalf, blocks[1].Num(), uint64(11))
	diff.Test(t, t.Fatalf, blocks[2].Num(), uint64(12))
	diff.Test(t, t.Fatalf, blocks[3].Num(), uint64(13))
	diff.Test(t, t.Fatalf, blocks[4].Num(), uint64(14))
}

func TestGet_Cached(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		diff.Test(t, t.Fatalf, nil, err)
		switch {
		case strings.Contains(string(body), "eth_getBlockByNumber"):
			_, err := w.Write([]byte(block18000000JSON))
			diff.Test(t, t.Fatalf, nil, err)
		case strings.Contains(string(body), "eth_getLogs"):
			_, err := w.Write([]byte(logs18000000JSON))
			diff.Test(t, t.Fatalf, nil, err)
		}
	}))
	defer ts.Close()

	c := New(ts.URL)
	blocks, err := c.Get(&glf.Filter{UseBlocks: true, UseLogs: true}, 18000000, 1)
	diff.Test(t, t.Errorf, nil, err)
	diff.Test(t, t.Errorf, len(blocks[0].Txs[0].Logs), 1)

	blocks, err = c.Get(&glf.Filter{UseBlocks: true, UseLogs: true}, 18000000, 1)
	diff.Test(t, t.Errorf, nil, err)
	diff.Test(t, t.Errorf, len(blocks[0].Txs[0].Logs), 1)
}

func TestNoLogs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		diff.Test(t, t.Fatalf, nil, err)
		switch {
		case strings.Contains(string(body), "eth_getBlockByNumber"):
			_, err := w.Write([]byte(block1000001JSON))
			diff.Test(t, t.Fatalf, nil, err)
		case strings.Contains(string(body), "eth_getLogs"):
			_, err := w.Write([]byte(logs1000001JSON))
			diff.Test(t, t.Fatalf, nil, err)
		}
	}))
	defer ts.Close()

	c := New(ts.URL)
	blocks, err := c.Get(&glf.Filter{UseBlocks: true, UseLogs: true}, 1000001, 1)
	diff.Test(t, t.Errorf, nil, err)

	b := blocks[0]
	diff.Test(t, t.Errorf, len(b.Txs), 1)

	tx := blocks[0].Txs[0]
	diff.Test(t, t.Errorf, len(tx.Logs), 0)
}

func TestLatest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		diff.Test(t, t.Fatalf, nil, err)
		switch {
		case strings.Contains(string(body), "eth_getBlockByNumber"):
			_, err := w.Write([]byte(block18000000JSON))
			diff.Test(t, t.Fatalf, nil, err)
		case strings.Contains(string(body), "eth_getLogs"):
			_, err := w.Write([]byte(logs18000000JSON))
			diff.Test(t, t.Fatalf, nil, err)
		}
	}))
	defer ts.Close()

	c := New(ts.URL)
	blocks, err := c.Get(&glf.Filter{UseBlocks: true, UseLogs: true}, 18000000, 1)
	diff.Test(t, t.Errorf, nil, err)

	b := blocks[0]
	diff.Test(t, t.Errorf, b.Num(), uint64(18000000))
	diff.Test(t, t.Errorf, fmt.Sprintf("%.4x", b.Parent), "198723e0")
	diff.Test(t, t.Errorf, fmt.Sprintf("%.4x", b.Hash()), "95b198e1")
	diff.Test(t, t.Errorf, b.Time, eth.Uint64(1693066895))
	diff.Test(t, t.Errorf, fmt.Sprintf("%.4x", b.LogsBloom), "53f146f2")
	diff.Test(t, t.Errorf, len(b.Txs), 94)

	tx0 := blocks[0].Txs[0]
	diff.Test(t, t.Errorf, fmt.Sprintf("%.4x", tx0.Hash()), "16e19967")
	diff.Test(t, t.Errorf, fmt.Sprintf("%.4x", tx0.To), "fd14567e")
	diff.Test(t, t.Errorf, fmt.Sprintf("%s", tx0.Value.Dec()), "0")
	diff.Test(t, t.Fatalf, len(tx0.Logs), 1)

	l := blocks[0].Txs[0].Logs[0]
	diff.Test(t, t.Errorf, fmt.Sprintf("%.4x", l.Address), "fd14567e")
	diff.Test(t, t.Errorf, fmt.Sprintf("%.4x", l.Topics[0]), "b8b9c39a")
	diff.Test(t, t.Errorf, fmt.Sprintf("%x", l.Data), "01e14e6ce75f248c88ee1187bcf6c75f8aea18fbd3d927fe2d63947fcd8cb18c641569e8ee18f93c861576fe0c882e5c61a310ae8e400be6629561160d2a901f0619e35040579fa202bc3f84077a72266b2a4e744baa92b433497bc23d6aeda4")

	signer, err := tx0.Signer()
	diff.Test(t, t.Errorf, nil, err)
	diff.Test(t, t.Errorf, fmt.Sprintf("%.4x", signer), "16d5783a")

	tx3 := blocks[0].Txs[3]
	diff.Test(t, t.Errorf, fmt.Sprintf("%s", tx3.Value.Dec()), "69970000000000014")
}
