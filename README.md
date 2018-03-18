# twopc_sort

实现 twopc_sort 的 sort 函数。
sort 函数功能定义如下
假设 dataStreamings 的数据是无尽的，即有无穷个 data 会产生；为了方便运行代码中设置了最大消息个数，但是假设的前提不变，必须满足！
按照 commit 值的大小对所有 kind 为 commit 的数据进行**排序并且输出**

实现目标：
请用你能想到**最快的排序**方式实现 sort 代码
请为 sort 代码部分实现**流控**

# 思路

`maxSleepInterval = 5`

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
和重要注释假定      
`assume max difference of send time between prepare and commit data is 2*maxSleepInterval(millisecond)`    

得到以下特征： 
   
- 每条(prepare类型**不可以扩大commit类型**)消息在`2*maxSleepInterval(millisecond)`内一定会完成发送到channel。 (commit消息由于CPU调度等原因被阻塞多久,没法假定)      
- 先读取到`Px`(prepare类型)消息，然后顺序读取若干条消息后，读取到`Py`(prepare类型)消息     
```     
if Py.sendTime - Px.sendTime >= 2*maxSleepInterval    
```     
则 `Py`之后从channel接收到的(prepare类型)消息的sendTime一定比`Px`的sendTime大 (**结论A**)-----可用于确定(prepare类型)消息sendTime的窗口    

- `Px`:表示 x事务的prepare类型的msg     
- `Cx`:表示 x事务的commit类型的msg    
- `Py`:表示 y事务的prepare类型的msg   
- `Cy`:表示 y事务的commit类型的msg    

- `Cx.sendTime <= Px.sendTime + 2*maxSleepInterval `: 表示 x下标的prepare类型的msg 的sendTime比 x下标的commit类型的msg 的sendTime最大差值为`2*maxSleepInterval`(**注释假定**)    

- `Cx.sendTime < Py.sendTime`，可以得到`Cx.commit < Cy.commit` (**结论B**)    
- 则 `Px.sendTime + 2*maxSleepInterval < Py.sendTime`就可得到 `Cx.commit < Cy.commit` : (**结论C**)
 

## 关于sort
go的sort库的sort函数，在数据量较大时（大于12时），会选使用quickSort,当分割的深度恶化时，改用heapSort，数据量小于或等于12时，使用insertSort(比递归更快)。 俺就不重新写轮子了。

## 关于flow control 
本来使用了`golang.org/x/time/rate`的代码，令牌桶的算法，本来想直接用go的tick来构造一个简单的，但是高性能情况下性能会不好，毕竟要定时器频繁调用。     
简单写了个`simpleLimiter`来替换原来的第三方库

## 方案一 （作废）

##### 作废原因：由于commit类型消息无法假定在`2*maxSleepInterval`完成channel写


- `Cx`:表示 x下标的commit类型的msg    
- `Py`:表示 y下标的prepare类型的msg   
- `Cx.sendTime - 2*maxSleepInterval <= Px.sendTime`: 表示 x下标的prepare类型的msg 的sendTime比 x下标的commit类型的msg 的sendTime最大差值为`2*maxSleepInterval`    
- `Px.sendTime < Cx.commit < Cx.sendTime`: 表示Cx.commit一定发生在该条msg的两个sendTime之间     
- 由上面两条可得   
`Cx.sendTime - 2*maxSleepInterval < Cx.commit < Cx.sendTime`    

- sendTime大于(1000+2*maxSleepInterval) ms的msg的commit 总是大于 sendTime=1000ms的msg的commit 

- 由于上面的**结论A**， 当把时间窗口增加`2*maxSleepInterval`时，后面从channel中读取的msg的sendTime都一定比本条msg的sendTime要大(**作废: commit类型消息无法满足**)

- 对每个chan的输出的msg(过滤掉prepare消息,后续可以考虑参与分块判断，加速分块)按照`2*maxSleepInterval`分块     
    `分块内的每条msg.sendTime <= 该分块(第一个消息的sendTime+2*maxSleepInterval)`  
	`m+3`分块内的每条msg.commit > `m`分块内的每条msg.commit  

- 块内用sort按照commit排序， 
- 相邻3个块，归并输出(归并窗口为3个块)，当最前面的块消耗完后，生成新的块，归并窗口后移，循环
- 对每个chan排序输出的结果(chan)再归并输出为总排序结果


## 方案二   
   
- 先读取到`Px`(prepare类型)消息，然后顺序读取若干条消息后，读取到`Py`(prepare类型)消息     
```     
if Py.sendTime - Px.sendTime >= 2*maxSleepInterval    
```     
则 `Py`之后从channel接收到的(prepare类型)消息的sendTime一定比`Px`的sendTime大 (**结论A**)

- `Px`:表示 x事务的prepare类型的msg     
- `Cx`:表示 x事务的commit类型的msg    
- `Py`:表示 y事务的prepare类型的msg   
- `Cy`:表示 y事务的commit类型的msg    

- `Cx.sendTime <= Px.sendTime + 2*maxSleepInterval `: 表示 x下标的prepare类型的msg 的sendTime比 x下标的commit类型的msg 的sendTime最大差值为`2*maxSleepInterval`(**注释假定**)    

- `Cx.sendTime < Py.sendTime`，可以得到`Cx.commit < Cy.commit` (**结论B**)    
- 则 `Px.sendTime + 2*maxSleepInterval < Py.sendTime`就可得到 `Cx.commit < Cy.commit` : (**结论C**)   

##### 思路1：      
对于接收到的msg按照sendTime排序结果如下
```   
p_1, p_3, c_1,p_2,c_2,c_3 ...
```	   
比c_1的commit小的消息只可能是 c_3    (**结论B**)

可以利用上面特性来确定比较窗口。按照sendTime排序，不一定比直接给commit排序来的简单。

##### 思路2：     

参考方案一，利用  (**结论A**) 和 (**结论C**), 对每个chan的输出的**prepare类型**msg按照`2*maxSleepInterval`分块：      
1. 分块内的每条msg.sendTime <= 该分块(第一个消息的sendTime+2*maxSleepInterval)     
2. `m+3`分块内的每条prepare消息对应的commit消息.commit > `m`分块内的每条prepare消息对应的commit消息.commit     
3. 找到块内prepare消息对应的commit消息(这一步较为麻烦,对应的commit消息可能在很后面，虽然commit消息的sendTime符合假定)，然后执行类似**方案一**的操作

没有下面这条补充假定，排序的窗口大小就没法较好的处理？
- 每条(**commit类型**)消息在`2*maxSleepInterval(millisecond)`内一定会完成发送到channel  


