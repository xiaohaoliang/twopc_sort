# twopc_sort

实现 twopc_sort 的 sort 函数。
sort 函数功能定义如下
假设 dataStreamings 的数据是无尽的，即有无穷个 data 会产生；为了方便运行代码中设置了最大消息个数，但是假设的前提不变，必须满足！
按照 commit 值的大小对所有 kind 为 commit 的数据进行**排序并且输出**

实现目标：
请用你能想到**最快的排序**方式实现 sort 代码
请为 sort 代码部分实现**流控**

# 思路

goroutine 循环逻辑
```
 1. generate prepare
 2. sleep 0~5 ms
 3. send prepare msg to channel
 4. sleep 0~5 ms
 5. generate commit
 6. sleep 0~5 ms
 7. send commit msg to channel
 8. sleep 0~50ms --> 1
```

分析code得到以下特征： 
- 对于同一个channel里面的msg的sendTime总是递增的
- sendTime大于1005ms的msg的commit 总是大于 sendTime=1000ms的msg的commit (方案一的排序窗口)
- TODO 方案二	

## 方案一 
使用了golang的sort库和rate库

- 对每个chan的输出的msg(过滤掉prepare消息)按照5ms分块
- 块内用sort按照commit排序， 
- 相邻两个块，归并输出(归并窗口为2个块)，当前面的块消耗完后，生成新的块，归并窗口后移，循环
- 对每个chan排序输出的结果(chan)再归并输出为总排序结果

### 问题：
- 按照sendTime切块，每次浮点数计算比较消耗相对较大，以时间为切分单位时，切分力度可能会大 
