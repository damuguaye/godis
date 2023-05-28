## 简易Redis
 - fork自[godis](https://github.com/archeryue/godis)
 - 完善了zset等数据结构
 - 加入了多个命令，其他文件也进行了一定的修改
## 使用流程
 - 进入文件目录，使用**go build**编译文件
 - 可通过**config.json**设置端口
 - 使用**myGodis config.json**开启redis-server
 - 打开另一终端，利用**telnet 127.0.0.1 6767**或者官方redis-cli连接server，按需修改ip与port
 - 连接成功后即可执行redis命令

# 以下为原项目README.md

## 项目背景
 - 本项目是用golang写的一个简略版本的redis-server，目的是用来讲解redis核心的技术原理。
 - 没有使用net库、goroutine、channel等golang特色工具。使用unix包的系统调用实现ae事件库，目的是为了复刻redis的设计。
 - ae事件库仅实现了epoll版本，所以只能在linux系统中运行。
 - 项目设计与实现中没有任何稳定性和性能等实用方面的考虑。
 - 目前版本有大量命令和功能没有实现，有兴趣的同学可以参考视频part9自行拓展。
## 视频讲解
 - Part 1：[Redis核心概念介绍](https://www.bilibili.com/video/BV1Zd4y1d7LY/)
 - Part 2：[Redis核心流程(ae循环)](https://www.bilibili.com/video/BV1HG4y1k7pH/)
 - Part 3：[Redis核心数据结构](https://www.bilibili.com/video/BV1sd4y1z7ir/)
 - Part 4：[Godis代码结构与Main函数](https://www.bilibili.com/video/BV1fe4y1i7rf/)
 - Part 5：[GodisAe库与Epoll封装](https://www.bilibili.com/video/BV1Hd4y117sL/)
 - Part 6：[Redis协议与ReadQueryFromClient](https://www.bilibili.com/video/BV1nd4y1c76f/)
 - Part 7：[List实现与SendReplyToClient](https://www.bilibili.com/video/BV1iv4y1U7sY/)
 - Part 8：[Dict实现与渐进式Rehash](https://www.bilibili.com/video/BV1c84y1C7wQ/)
 - Part 9：[命令实现](https://www.bilibili.com/video/BV19Y41117Yy/)
