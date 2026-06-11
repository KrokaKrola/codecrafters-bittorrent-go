package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"sync"
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

func worker(btFile *bitTorrentFile, id int, wg *sync.WaitGroup, in <-chan job, results chan<- jobSuccessResult, failures chan<- jobFailure) {
	fmt.Println("Starting worker id=", id)
	defer wg.Done()

	peer, err := handshakeWithPeer(btFile, btFile.peers[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer peer.conn.Close()

	if err := unchokePeer(peer); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for job := range in {
		fmt.Println("requesting piece", job.pieceIdx)
		piece, err := getPiece(btFile, peer, job.pieceIdx)
		if err != nil {
			failures <- jobFailure{
				retryCount: job.retryCount + 1,
				pieceIdx:   job.pieceIdx,
				err:        err,
			}
			continue
		}

		fmt.Println("got piece", job.pieceIdx)

		pieceHash, err := createSha1Hash(piece, true)
		if err != nil {
			failures <- jobFailure{
				retryCount: job.retryCount + 1,
				pieceIdx:   job.pieceIdx,
				err:        fmt.Errorf("error creating hash for received blocks"),
			}
			continue
		}

		if pieceHash != hex.EncodeToString(btFile.info.piecesParts[job.pieceIdx]) {
			failures <- jobFailure{
				retryCount: job.retryCount + 1,
				pieceIdx:   job.pieceIdx,
				err:        fmt.Errorf("piece hash is invalid"),
			}
			continue
		}

		results <- jobSuccessResult{
			pieceIdx: job.pieceIdx,
			result:   piece,
		}
	}
}

func createPool(btFile *bitTorrentFile, count int, in <-chan job) (<-chan jobSuccessResult, <-chan jobFailure) {
	var wg sync.WaitGroup
	results := make(chan jobSuccessResult)
	failures := make(chan jobFailure)

	for i := range count {
		wg.Add(1)
		go worker(btFile, i, &wg, in, results, failures)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results, failures
}

func downloadFile(btFile *bitTorrentFile, filePath string, file *os.File) error {
	if err := os.Truncate(filePath, int64(btFile.info.length)); err != nil {
		return fmt.Errorf("Error preallocating file on the disk %s", filePath)
	}

	jobs := make(chan job)

	go func() {
		for pieceIdx := range btFile.info.piecesParts {
			fmt.Println("add piece idx", pieceIdx, "to the jobs chan")
			jobs <- job{pieceIdx: pieceIdx}
		}
	}()

	results, failures := createPool(btFile, WorkersCount, jobs)
	var wg sync.WaitGroup
	wg.Add(len(btFile.info.piecesParts))

	go func() {
		for jobRes := range results {
			if _, err := file.WriteAt(jobRes.result, int64(jobRes.pieceIdx)*int64(btFile.info.pieceLength)); err != nil {
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
				jobs <- job{
					pieceIdx:   failure.pieceIdx,
					retryCount: failure.retryCount,
				}
			}
		}
	}()

	wg.Wait()
	fmt.Println("File saved to", filePath)

	return nil
}
