package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/codecrafters-io/bittorrent-starter-go/app/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/app/magnet"
	"github.com/codecrafters-io/bittorrent-starter-go/app/torrent"
	"github.com/codecrafters-io/bittorrent-starter-go/app/utils/stringutil"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Invalid number of arguments")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	// torrent commands
	case "download":
		// ./your_program.sh download -o /tmp/test.txt sample.torrent
		if len(os.Args) < 5 {
			fmt.Println("Invalid number of arguments")
			os.Exit(1)
		}

		torrentFileName := os.Args[4]

		if torrentFileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		filePath := os.Args[3]

		if filePath == "" {
			fmt.Println("File path is empty")
			os.Exit(1)
		}

		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			fmt.Println("Error reading file", filePath)
			os.Exit(1)
		}

		defer file.Close()

		btFile, err := torrent.NewTorrentFile(torrentFileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := btFile.LoadPeers(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if len(btFile.Peers) == 0 {
			fmt.Println("no peers for torrent file", torrentFileName)
			os.Exit(1)
		}

		if err := btFile.DownloadFile(filePath, file); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
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

		btFile, err := torrent.NewTorrentFile(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := btFile.LoadPeers(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if len(btFile.Peers) == 0 {
			fmt.Println("no peers for torrent file", fileName)
			os.Exit(1)
		}

		peerConn, err := btFile.Peers[0].HandshakeWithPeer(btFile.ShaInfoHash, false)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		defer peerConn.Close()

		if err := btFile.Peers[0].UnchokePeer(peerConn); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		piece, err := btFile.GetPiece(peerConn, pieceIndex)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		pieceHash, err := stringutil.CreateSha1Hash(piece, true)
		if err != nil {
			fmt.Println("error creating hash for received blocks")
			os.Exit(1)
		}

		if pieceHash != hex.EncodeToString(btFile.Info.PiecesParts[pieceIndex]) {
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

		btFile, err := torrent.NewTorrentFile(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := btFile.LoadPeers(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		peerConn, err := btFile.Peers[0].HandshakeWithPeer(btFile.ShaInfoHash, false)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		defer peerConn.Close()

		fmt.Println("Peer ID:", hex.EncodeToString(btFile.Peers[0].Id))
	case "peers":
		fileName := os.Args[2]

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		btFile, err := torrent.NewTorrentFile(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := btFile.LoadPeers(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		for _, el := range btFile.Peers {
			fmt.Println(el)
		}
	case "info":
		fileName := os.Args[2]

		if fileName == "" {
			fmt.Println("Empty file name")
			os.Exit(1)
		}

		btFile, err := torrent.NewTorrentFile(fileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("Tracker URL:", btFile.Announce)
		fmt.Println("Length:", btFile.Info.Length)
		fmt.Println("Info Hash:", hex.EncodeToString([]byte(btFile.ShaInfoHash)))
		fmt.Println("Piece Length:", btFile.Info.PieceLength)
		fmt.Println("Piece Hashes:")
		for _, el := range btFile.Info.PiecesParts {
			fmt.Println(hex.EncodeToString(el))
		}
	case "decode":
		bencodedValue := os.Args[2]

		dec := bencode.Decoder{
			Data: []byte(bencodedValue),
		}

		decoded := dec.Decode()

		if dec.Err != nil {
			fmt.Println(dec.Err)
			return
		}

		if _, ok := decoded.(bencode.Dictionary); ok {
			decoded = decoded.(bencode.Dictionary).ToMap()
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))

		// magnet commands
	case "magnet_parse":
		magnetLink := os.Args[2]

		magnet, err := magnet.NewMagnet(magnetLink)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("Tracker URL:", magnet.TrackerUrl)
		fmt.Println("Info Hash:", hex.EncodeToString(magnet.InfoHash))
	case "magnet_handshake":
		magnetLink := os.Args[2]

		magnet, err := magnet.NewMagnet(magnetLink)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		peer := magnet.Peers[0]

		peerConn, err := peer.HandshakeWithPeer(string(magnet.InfoHash), true)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		defer peerConn.Close()

		if err := peer.DoExtensionHandshake(peerConn); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("Peer ID:", hex.EncodeToString(magnet.Peers[0].Id))
		fmt.Println("Peer Metadata Extension ID:", peer.MetaDataId)
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
