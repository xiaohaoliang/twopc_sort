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
- 对于同一个channel里面的msg：
	commit消息之后接收到的prepare消息对应的commit消息的commit， 一定大于前面的commit消息。(方案二的排序窗口)

## 关于sort
go的sort库的sort函数，在数据量较大时（大于12时），会选使用quickSort,当分割的深度恶化时，改用heapSort，数据量小于或等于12时，使用insertSort(比递归更快)。 俺就不重新写轮子了。

## 关于flow control 
本来使用了`golang.org/x/time/rate`的代码，令牌桶的算法，本来想直接用go的tick来构造一个简单的，但是高性能情况下性能会不好，毕竟要定时器频繁调用。     
简单写了个`simpleLimiter`来替换原来的第三方库

## 方案一 
使用了golang的sort库和rate库

- 对每个chan的输出的msg(过滤掉prepare消息)按照5ms分块
- 块内用sort按照commit排序， 
- 相邻两个块，归并输出(归并窗口为2个块)，当前面的块消耗完后，生成新的块，归并窗口后移，循环
- 对每个chan排序输出的结果(chan)再归并输出为总排序结果

## 方案二   

对于同一个chan的msg，msg的sendTime总是**递增的**。    
- `Cx`:表示 x下标的commit类型的msg    
- `Py`:表示 y下标的prepare类型的msg   
- `Cx.sendTime < Py.sendTime`: 表示 x下标的commit类型的msg 比 y下标的prepare类型的msg先发送。    
- `Cx.commit <= Cx.sendTime` 获取commit总是在该消息发送之前    
- `Py.sendTime <= Cy.commit` 该事务的prepare消息发送之后，才会获得该事务的commit      
- 可以得到`Cx.commit < Cy.commit`,即 commit消息之后接收到的prepare消息对应的commit消息的commit， 一定大于前面的commit消息。     

举例如下   

```   
p_1, p_3, c_1,p_2,c_2,c_3 ...
```	   

比c_1的commit小的消息只可能是 c_3    

可以利用上面特性来确定比较窗口， 不用频繁比较time类型。TODO