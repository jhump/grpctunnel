//go:build !debug

package grpctunnel

func logSend(stream string, chunkIndex int, chunkSize, totalMsgSize, windowRemaining uint32) {
	// no-op without debug build tag
}

func logSenderUpdate(stream string, ackSize, windowRemaining uint32) {
	// no-op without debug build tag
}

func logReceive(stream string, chunkSize uint, windowRemaining uint32) {
	// no-op without debug build tag
}

func logReceiverAck(stream string, chunkSize uint, outstandingAck, windowRemaining uint32, sending bool) {
	// no-op without debug build tag
}
