//go:build debug

package grpctunnel

import "log"

// debugEnabled is true when built with the "debug" build tag. It can be used
// to skip work, like formatting stream names, that is only needed for logging.
const debugEnabled = true

func logSend(stream string, chunkIndex int, chunkSize, totalMsgSize, windowRemaining uint32) {
	log.Printf("%s: sending chunk #%d, %d bytes (out of %d) -- window size: %d => %d",
		stream, chunkIndex+1, chunkSize, totalMsgSize, windowRemaining, int(windowRemaining)-int(chunkSize))
}

func logSenderUpdate(stream string, ackSize, windowRemaining uint32) {
	log.Printf("%s: receiving update acknowledging %d bytes -- window size: %d => %d",
		stream, ackSize, windowRemaining, windowRemaining+ackSize)
}

func logReceive(stream string, chunkSize uint, windowRemaining uint32) {
	log.Printf("%s: receiving chunk %d bytes -- window size: %d => %d",
		stream, chunkSize, windowRemaining, int(windowRemaining)-int(chunkSize))
}

func logReceiverAck(stream string, chunkSize uint, outstandingAck uint32, windowRemaining uint32, sending bool) {
	if sending {
		log.Printf("%s: acknowledged %d bytes; sending update for %d bytes -- window size: %d => %d",
			stream, chunkSize, outstandingAck, windowRemaining, windowRemaining+outstandingAck)
	} else {
		log.Printf("%s: acknowledged %d bytes (bytes outstanding: %d => %d) -- window size: %d",
			stream, chunkSize, int(outstandingAck)-int(chunkSize), outstandingAck, windowRemaining)
	}
}
