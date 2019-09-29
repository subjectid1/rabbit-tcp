package connection

import (
	"github.com/ihciah/rabbit-tcp/block"
	"sync"
)

// 1. Join blocks from chan to connection orderedRecvQueue
// 2. Send bytes or control block
type blockProcessor struct {
	conn            Connection
	cache           map[uint32]block.Block
	sendBlockIDLock sync.Mutex
	recvBlockIDLock sync.Mutex
	sendBlockID     uint32
	recvBlockID     uint32
}

func newBlockProcessor(conn Connection) blockProcessor {
	return blockProcessor{
		conn:        conn,
		cache:       make(map[uint32]block.Block),
		sendBlockID: 1,
		recvBlockID: 1,
	}
}

func (x *blockProcessor) RecvBlock(blk block.Block) {
	x.conn.GetRecvQueue() <- blk
}

func (x *blockProcessor) SendBlock(blk block.Block) {
	// Put block into connectionPool sendQueue
	x.conn.GetSendQueue() <- blk
}

func (x *blockProcessor) SendData(data []byte) {
	x.sendBlockIDLock.Lock()
	blocks := block.NewDataBlocks(x.conn.GetConnectionID(), x.sendBlockID, data)
	x.sendBlockID += uint32(len(blocks))
	x.sendBlockIDLock.Unlock()
	for _, blk := range blocks {
		x.SendBlock(blk)
	}
}

func (x *blockProcessor) SendConnect(address string) {
	x.sendBlockIDLock.Lock()
	blkID := x.sendBlockID
	x.sendBlockID += 1
	x.sendBlockIDLock.Unlock()

	blk := block.NewConnectBlock(x.conn.GetConnectionID(), blkID, address)
	x.SendBlock(blk)
}

func (x *blockProcessor) SendDisconnect() {
	x.sendBlockIDLock.Lock()
	blkID := x.sendBlockID
	x.sendBlockID += 1
	x.sendBlockIDLock.Unlock()

	blk := block.NewDisconnectBlock(x.conn.GetConnectionID(), blkID)
	x.SendBlock(blk)
}

// Join blocks and send buffer to connection
func (x *blockProcessor) Daemon(connection Connection) {
	for {
		blk := <-x.conn.GetRecvQueue()
		if x.recvBlockID == blk.BlockID {
			connection.GetOrderedRecvQueue() <- blk
			x.recvBlockID++
			for {
				blk, ok := x.cache[x.recvBlockID]
				if !ok {
					break
				}
				connection.GetOrderedRecvQueue() <- blk
				delete(x.cache, x.recvBlockID)
				x.recvBlockID++
			}
		} else {
			x.cache[blk.BlockID] = blk
		}
	}
}
