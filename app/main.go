package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Invalid number of arguments")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "download_piece":
		// ./your_program.sh download_piece -o /tmp/test-piece sample.torrent <piece_index>
		if len(os.Args) < 6 {
			fmt.Println("Invalid number of arguments")
			os.Exit(1)
		}

		fileName := os.Args[4]

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		pieceIndexStr := os.Args[5]
		if pieceIndexStr == "" {
			fmt.Println("piece_index is empty")
			os.Exit(1)
		}

		pieceIndex, err := strconv.Atoi(pieceIndexStr)
		if err != nil {
			fmt.Println("piece_index is not a number")
			os.Exit(1)
		}

		btFile, err := parseBitTorrentFile(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err = enrichWithPeers(btFile); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if len(btFile.peers) == 0 {
			fmt.Println("no peers for torrent file", fileName)
			os.Exit(1)
		}

		peer, err := handshakeWithPeer(btFile, btFile.peers[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		defer peer.conn.Close()

		piece, err := getPiece(btFile, peer, pieceIndex)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		pieceHash, err := createSha1Hash(piece, true)
		if err != nil {
			fmt.Println("error creating hash for received blocks")
			os.Exit(1)
		}

		if pieceHash != hex.EncodeToString(btFile.info.piecesParts[pieceIndex]) {
			fmt.Println("piece hash is invalid")
			os.Exit(1)
		}

		if err := os.WriteFile(os.Args[3], piece, 0644); err != nil {
			fmt.Println("error saving the file into", os.Args[3])
			os.Exit(1)
		}
	case "handshake":
		if len(os.Args) < 3 {
			fmt.Println("Invalid number of arguments")
			os.Exit(1)
		}
		fileName := os.Args[2]
		peerAddress := os.Args[3]

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		if peerAddress == "" {
			fmt.Println("Empty peer address")
			os.Exit(1)
		}

		btFile, err := parseBitTorrentFile(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		peer, err := handshakeWithPeer(btFile, peerAddress)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer peer.conn.Close()

		fmt.Println("Peer ID:", hex.EncodeToString(peer.id))
	case "peers":
		fileName := os.Args[2]

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		btFile, err := parseBitTorrentFile(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err = enrichWithPeers(btFile); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		for _, el := range btFile.peers {
			fmt.Println(el)
		}
	case "info":
		fileName := os.Args[2]

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		btFile, err := parseBitTorrentFile(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("Tracker URL:", btFile.announce)
		fmt.Println("Length:", btFile.info.length)
		fmt.Println("Info Hash:", hex.EncodeToString([]byte(btFile.shaInfoHash)))
		fmt.Println("Piece Length:", btFile.info.pieceLength)
		fmt.Println("Piece Hashes:")
		for _, el := range btFile.info.piecesParts {
			fmt.Println(hex.EncodeToString(el))
		}
	case "decode":
		bencodedValue := os.Args[2]

		dec := decoder{
			data: []byte(bencodedValue),
		}

		decoded := dec.decode()

		if dec.err != nil {
			fmt.Println(dec.err)
			return
		}

		if _, ok := decoded.(dictionary); ok {
			decoded = decoded.(dictionary).toMap()
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
