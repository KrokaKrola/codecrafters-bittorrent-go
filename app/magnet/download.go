package magnet

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/codecrafters-io/bittorrent-starter-go/app/utils/stringutil"
)

const (
	WorkersCount = 5
	MaxRetries   = 3
)

type job struct {
	pieceIdx   int
	retryCount int
}

type jobSuccessResult struct {
	pieceIdx int
	result   []byte
}

type jobFailure struct {
	pieceIdx   int
	retryCount int
	err        error
}

func (mg *Magnet) connectWorker() (net.Conn, error) {
	p := mg.Peers[0]

	conn, err := p.HandshakeWithPeer(string(mg.InfoHash), true)
	if err != nil {
		return nil, err
	}

	if err := p.DoExtensionHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	if _, err := p.DoMetadataRequest(conn); err != nil {
		conn.Close()
		return nil, err
	}

	if err := p.SendInterested(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (mg *Magnet) worker(id int, wg *sync.WaitGroup, in <-chan job, results chan<- jobSuccessResult, failures chan<- jobFailure) {
	fmt.Println("Starting worker id=", id)
	defer wg.Done()

	conn, err := mg.connectWorker()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer conn.Close()

	for job := range in {
		fmt.Println("requesting piece", job.pieceIdx)
		piece, err := mg.GetPiece(conn, job.pieceIdx)
		if err != nil {
			failures <- jobFailure{retryCount: job.retryCount + 1, pieceIdx: job.pieceIdx, err: err}
			continue
		}

		fmt.Println("got piece", job.pieceIdx)

		pieceHash, err := stringutil.CreateSha1Hash(piece, true)
		if err != nil {
			failures <- jobFailure{retryCount: job.retryCount + 1, pieceIdx: job.pieceIdx, err: fmt.Errorf("error creating hash for received blocks")}
			continue
		}

		if pieceHash != hex.EncodeToString(mg.PieceParts[job.pieceIdx]) {
			failures <- jobFailure{retryCount: job.retryCount + 1, pieceIdx: job.pieceIdx, err: fmt.Errorf("piece hash is invalid")}
			continue
		}

		results <- jobSuccessResult{pieceIdx: job.pieceIdx, result: piece}
	}
}

func (mg *Magnet) createPool(count int, in <-chan job) (<-chan jobSuccessResult, <-chan jobFailure) {
	var wg sync.WaitGroup
	results := make(chan jobSuccessResult)
	failures := make(chan jobFailure)

	for i := range count {
		wg.Add(1)
		go mg.worker(i, &wg, in, results, failures)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results, failures
}

func (mg *Magnet) DownloadFile(filePath string, file *os.File) error {
	if err := os.Truncate(filePath, int64(mg.Length)); err != nil {
		return fmt.Errorf("error preallocating file on disk %s", filePath)
	}

	jobs := make(chan job)

	go func() {
		for pieceIdx := range mg.PieceParts {
			fmt.Println("add piece idx", pieceIdx, "to the jobs chan")
			jobs <- job{pieceIdx: pieceIdx}
		}
	}()

	results, failures := mg.createPool(WorkersCount, jobs)
	var wg sync.WaitGroup
	wg.Add(len(mg.PieceParts))

	go func() {
		for jobRes := range results {
			if _, err := file.WriteAt(jobRes.result, int64(jobRes.pieceIdx)*int64(mg.PieceLength)); err != nil {
				fmt.Println("error writing job result to file;", "for pieceIdx=", jobRes.pieceIdx, err)
				os.Exit(1)
			}
			wg.Done()
		}
	}()

	go func() {
		for failure := range failures {
			fmt.Printf("pieceIdx=%d failed %d time with error=%s\n", failure.pieceIdx, failure.retryCount, failure.err.Error())
			if failure.retryCount < MaxRetries {
				jobs <- job{pieceIdx: failure.pieceIdx, retryCount: failure.retryCount}
			}
		}
	}()

	wg.Wait()
	fmt.Println("File saved to", filePath)
	return nil
}
