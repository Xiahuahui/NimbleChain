## Tendermint中的Mconnection

参数的解释

    defaultMaxPacketMsgPayloadSize = 1024 //每个数据包的大小
	numBatchPacketMsgs = 10   //TODO:
	minReadBufferSize  = 1024 //TODO:
	minWriteBufferSize = 65536
	updateStats        = 2 * time.Second

	// some of these defaults are written in the user config
	// flushThrottle, sendRate, recvRate
	// TODO: remove values present in config
	defaultFlushThrottle = 100 * time.Millisecond

	defaultSendQueueCapacity   = 1                //TODO： 通道中发送数据的队列的长度
	defaultRecvBufferCapacity  = 4096             //TODO：
	defaultRecvMessageCapacity = 22020096         // 21MB  //TODO：
	defaultSendRate            = int64(512000)    // 500KB/s
	defaultRecvRate            = int64(512000)    // 500KB/s
	defaultSendTimeout         = 10 * time.Second //将消息放通道到相应队列的最长等待时间 （TODO 需要加个日志  阻塞的）
	defaultPingInterval        = 60 * time.Second
	defaultPongTimeout         = 45 * time.Second

Mconnect的成员函数的解释

    send（）：
    将要发送的数据byte[],放入对应的channel，发送的channel中，
    这块应该是每个peer对应n个channel，每个peer对应一个协程。
    对应发送区块的情况，广播，每个协程负责发送一个区块。是否和机器的core有关系？？？



