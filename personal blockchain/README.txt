区块链主体是两个文件，main.go 和 transaction.go，非常抱歉没有分成几个文件或者是用接口实现，因为我在我的电脑上钻研很久发现IDE怎么编译都会错，在终端编译又会需要输入很多文件名，所以干脆放一个文件里面了。每一块函数的作用我都用注释表明了。

要运行区块链，需要在文件夹当前目录下进入终端，模拟3个端口通信，运行如下指令
go run main.go transaction.go -address=localhost:8080 -peers=localhost:8081,localhost:8082
新开一个终端窗口，运行如下指令
go run main.go transaction.go -address=localhost:8081 -peers=localhost:8080,localhost:8082
新开一个终端窗口，运行如下指令
go run main.go transaction.go -address=localhost:8082 -peers=localhost:8080,localhost:8081

数据存储我使用json格式

体现独特设计这块我是简单的尝试了一个分片技术，想法是将交易按地址哈希分片，每个节点只需验证部分交易。实现是定义分片规则，例如根据 Sender 地址哈希值决定交易属于哪个分片。节点只需要存储并验证属于自己分片的交易和区块。在同步时只广播与本分片相关的数据。
这一部分因为需要改不少核心的代码，所以我单独重新放了一份独特设计代码，在目录下的\unique文件夹下。

重新浏览要求的时候发现我的区块链主体代码没有使用Merkle根，我把有Merkle根的版本更新在目录下的\unique文件夹的版本里面了。
