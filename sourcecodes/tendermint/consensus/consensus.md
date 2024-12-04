[//]: # (只要超过2/3的的投票肯定要进入到相应的round中)

进入newheight的情况：

    1.区块提交后进入新的高度；

进入newround的情况：

    1.当进入新的高度并且超时时；
    2.拿到所有的lastcommit的投票时，（并且配置中可以跳过timeoutCommit）；
    3.当拿到未来轮次的prevote投票，并且超过了2/3 但是不用大多数（大多数节点其实已经进入未来轮次了，所以本轮不用再等了）

进入propose的情况：

    1.当newRound超时时；
    2.当在NewRound时，不需要等待可用的tx，也就是可以产生空块的时候；
    3.当tx可用时，并且在NewRound时

进入prevote的情况：

     1.当自己是leader时，存在pol时，直接进入prevote阶段；
     2.当propose阶段超时时；
     3.当收到完整区块时；
     4.当存在pol时，收到了最后一个vote TODO (也是包含未来的意思)

进入prevotewait的情况:

    1.当收到2/3以上的prevote，并且不能达成一致的情况

进入precommit的情况：

    1.当在prevotewait出现超时时；
    2.当在precommitwait出现超时；（这个其实对应的是空的区块）；
    3.当先拿到2/3的prevote投票，后在拿到完整的区块时；
    4.当先拿到区块，后拿到2/3的prevote的投票时；
    5.当拿到未来轮次的2/3的投票时

进入precommitwait的情况：

进入commit的情况：
