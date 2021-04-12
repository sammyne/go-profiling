# Go 性能调优

本文主要讲解 go 程序的性能测评，包括 pprof、火焰图和 trace 图的使用，进而通过测评结果指导调优方向。

> 演示环境为 macOS Big Sur 11.2.2 下的 Go 1.16.3。

## runtime/pprof
pprof 是 go 官方提供的性能测评工具，包含在 net/http/pprof 和 runtime/pprof 两个包，分别用于不同场景

- runtime/pprof 主要用于可结束的代码块，如一次编解码操作等
- net/http/pprof 是对 runtime/pprof 的二次封装，主要用于不可结束的代码块，如 web 应用等

我们先看看如何利用 runtime/pprof 进行性能测评。

下列代码循环向一个列表尾部添加元素。导入 runtime/pprof 并添加两段测评代码（补充具体行号）就可以实现 CPU 和内存的性能评测。

```
TODO
```

编译并执行获得 pprof 的采样数据，然后利用相关工具进行分析。

```bash
$:go build main.go
$:./main --cpuprofile=cpu.pprof
$:./main --memprofile=mem.pprof
$:go tool pprof cpu.pprof
```

至此就可以获得 cpu.pprof 和 mem.pprof 两个采样文件，然后利用 `go tool pprof` 工具进行分析如下。

```bash
➜  code git:(main) ✗ go tool pprof cpu.pprof 
Type: cpu
Time: Apr 7, 2021 at 2:02pm (CST)
Duration: 201.17ms, Total samples = 130ms (64.62%)
Entering interactive mode (type "help" for commands, "o" for options)
(pprof) top
Showing nodes accounting for 130ms, 100% of 130ms total
Showing top 10 nodes out of 41
      flat  flat%   sum%        cum   cum%
      40ms 30.77% 30.77%       40ms 30.77%  runtime.memmove
      20ms 15.38% 46.15%       20ms 15.38%  runtime.madvise
      20ms 15.38% 61.54%       20ms 15.38%  runtime.pthread_kill
      10ms  7.69% 69.23%       70ms 53.85%  main.counter (inline)
      10ms  7.69% 76.92%       10ms  7.69%  runtime.pcdatavalue
      10ms  7.69% 84.62%       10ms  7.69%  runtime.pthread_cond_timedwait_relative_np
      10ms  7.69% 92.31%       10ms  7.69%  runtime.scanobject
      10ms  7.69%   100%       10ms  7.69%  runtime.usleep
         0     0%   100%       70ms 53.85%  main.workOnce
         0     0%   100%       10ms  7.69%  runtime.(*gcControllerState).enlistWorker
(pprof) 
```

相关字段如下（`Type` 和 `Time` 字段就不过多解释了）：

|       字段 | 说明                                                                                                                                                    |
| ---------: | :------------------------------------------------------------------------------------------------------------------------------------------------------ |
|   Duration | 程序执行时间。本例中 go 自动分配任务给多个核执行程序，总计耗时 201.17ms，而采样时间为 130ms；也就是说假设有 10 核执行程序，平均每个核采样 13ms 的数据。 |
|    (pprof) | 命令行提示。表示当前正在执行 go 的 pprof 工具命令行中，其他工具有 cgo、doc、pprof、test2json、trace 等                                                  |
|        top | pprof 的指令之一，显示 pprof 文件的前 10 项数据，可以通过 top 20 等方式显示前 20 行数据。pprof 还有很多指令，例如 list、pdf、eog 等等                   |
| flat/flat% | 分别表示在当前层级的 CPU 占用时间和百分比。例如 `runtime.memmove` 在当前层级占用 CPU 时间 40ms，占比本次采集时间的 30.77%                               |
|   cum/cum% | 分别表示截止到当前层级累积的 CPU 时间和占比。例如 `main.counter` 累积占用时间 7ms，占本次采集时间的 53.85%                                              |
|       sum% | 所有层级的 CPU 时间累积占用，从小到大一直累积到 100%，即 130ms                                                                                          |

由上图的 `cum` 数据可以看到，`counter` 函数的 CPU 占用时间最多。接下来可利用 `list` 命令查看占用的主要因素如下

```bash
(pprof) list main.counter
Total: 130ms
ROUTINE ======================== main.counter in /Users/lixiangmin01/Desktop/go-profiling/code/counter_v1.go
      10ms       70ms (flat, cum) 53.85% of Total
         .          .     50:}
         .          .     51:
         .          .     52:func counter() {
         .          .     53:	slice := make([]int, 0)
         .          .     54:	var c int
      10ms       10ms     55:	for i := 0; i < 100000; i++ {
         .          .     56:		c = i + 1 + 2 + 3 + 4 + 5
         .       60ms     57:		slice = append(slice, c)
         .          .     58:	}
         .          .     59:	_ = slice
         .          .     60:}
         .          .     61:
         .          .     62:func workOnce(wg *sync.WaitGroup) {
(pprof) 
```

可见，程序的 55 行和 57 行分别占用 10ms 和 60ms，这就是优化的主要方向。通过分析程序发现，由于切片的初始容量为 0，导致循环 `append` 时触发多次扩容。切片的扩容方式是：申请 2 倍或者 1.25 倍的原来长度的新切片，再将原来的切片数据拷贝进去。

仔细一看还会发现：`runtime.usleep` 占用 CPU 时间将近 20%，但是程序明明没有任何 sleep 相关的代码，却为什么会出现，并且还占用这么高呢？大家可以先思考一下，后文将揭晓。

当然，也可以使用 `web` 命令获得更加直观的信息。macOS 通过如下命令安装渲染工具 graphviz。

```bash
brew install graphviz
```

安装完成后，在 pprof 的命令行输入 `web` 即可生成一个 svg 格式的文件，将其用浏览器打开即可看到如下界面：

TODO

由于文件过大，此处只截取部分重要内容如下。

可以看出其基本信息和命令行下的信息相同，但是可以明显看出 `runtime.memmove` 耗时 380ms。由图逆向推断 `main.counter` 是主要的优化方向。图中各个方块的大小也代表 CPU 占用的情况，方块越大说明占用 CPU 时间越长。

同理可以分析 mem.pprof 文件，从而得出内存消耗的主要原因进一步进行改进。

上述 `main.counter` 占用 CPU 时间过多的问题，实际上是 `append` 函数重新分配内存造成的。那简单的做法就是事先申请一个大的内存，避免频繁的进行内存分配。所以将 `counter` 函数进行改造：

```
TODO
```

通过编译、运行、采集 pprof 信息后如下图所示。

可见，已经采集不到占用 CPU 比较多的函数，即已经完成优化。同学们可以试试如果往 `counter` 添加一个 `fmt.Println` 函数后，对 CPU 占用会有什么影响呢？

## net/http/pprof

针对后台服务型应用，服务一般不能停止，这时需要使用 net/http/pprof 包。类似上述代码，编写如下代码：

```
TODO
```

首先导入 net/http/pprof 包。注意该包利用下划线 `_` 导入，意味着只需要该包运行其 `init()` 函数即可。这样之后，该包将自动完成信息采集并保存到内存。所以服务上线时需要将 net/http/pprof 包移除，避免其影响服务的性能，更重要的是防止其造成内存的不断上涨。

编译并运行依赖，便可以访问：http://localhost:8000/debug/pprof/ 查看服务的运行情况。本文实验得出如下示例，大家可以自行探究查看。不断刷新网页可以发现采样结果也在不断更新中。

TODO

当然也可以网页形式查看。现在以查看内存为例，在服务程序运行时，执行下列命令采集内存信息。

```bash
go tool pprof main http://localhost:8000/debug/pprof/heap
```

采集完成后调用 web 命令得到如下 svg 文件

TODO

该图表明所有的堆空间均由 `counter` 产生，同理可以生成 CPU 的 svg 文件用于同步进行分析优化方向。

上述方法在工具型应用可以使用，然而在服务型应用时，仅仅只是采样了部分代码段；而只有当有大量请求时才能看到应用服务的主要优化信息。

另外，Uber 开源的火焰图工具 go-torch 能够辅助我们直观地完成测评。要想实现火焰图的效果，需要安装如下 3 个工具：

go get -v github.com/uber/go-torch
git clone https://github.com/brendangregg/FlameGraph.git
git clone https://github.com/wg/wrk
其中下载FlameGraph和wrk后需要进行编译，如果需要长期使用，需要将二者的可执行文件路径放到系统环境变量中。FlameGraph是画图需要的工具，而wrk是模拟并发访问的工具。通过如下命令进行模拟并发操作：500个线程并发，每秒保持2000个连接，持续时间30s

./wrk -t500 -c2000 -d30s http://localhost:8000/get
同时开启go-torch工具对http://localhost:8000采集30s信息，采集完毕后会生成svg的文件，用浏览器打开就是火焰图，如下所示：

go-torch -u http://localhost:8000 -t 30

火焰图形似火焰，故此得名，其横轴是CPU占用时间，纵轴是调用顺序。由上图可以看出main.counter占用将近50%的CPU时间。通过wrk的压测后，我们可以再查看内存等信息，

go tool pprof main http://localhost:8000/debug/pprof/heap //采集内存信息
go tool pprof main http://localhost:8000/debug/pprof/profile //采集cpu信息
利用web指令看到内存的使用情况如下。其中counter函数占用67.20%，且包含2部分，因为我们的代码中有2处调用counter函数。如果大家觉得web框图更加清晰，完全可以摒弃火焰图，直接使用go tool pprof工具。


针对上述分析，我们同样通过分配初始内存，降低内存扩容次数方法进行优化。即将counter函数修改成与上文所示，再次进行cpu和内存的性能评测，火焰图和web框图分别如下：



从上面的两幅图中可以看到，cpu和堆空间的使用大大降低；同时在web框图中看到pprof也会使用堆空间，所以在服务上线时应该将pprof关闭。

## trace
trace工具也是golang支持的go tool工具之一，能够辅助我们跟踪程序的执行情况，进一步方便我们排查问题，往往配合pprof使用。trace的使用和pprof类似，为了简化分析，我们首先利用下列代码进行讲解，只是用1核运行程序：

package main

import (
    "os"
    "runtime"
    "runtime/trace"
    "sync"
    "flag"
    "log"
)

func counter(wg *sync.WaitGroup) {
    wg.Done()
    slice := []int{0}
    c := 1
    for i := 0; i < 100000; i++ {
        c = i + 1 + 2 + 3 + 4 + 5
        slice = append(slice, c)
    }
}

func main(){
    runtime.GOMAXPROCS(1)
    var traceProfile = flag.String("traceprofile", "", "write trace profile to file")
    flag.Parse()

    if *traceProfile != "" {
        f, err := os.Create(*traceProfile)
        if err != nil {
            log.Fatal(err)
        }
        trace.Start(f)
        defer f.Close()
        defer trace.Stop()
    }

  var wg sync.WaitGroup
    wg.Add(3)
    for i := 0; i < 3; i ++ {
        go counter(&wg)
    }
    wg.Wait()
}
同样，通过编译、执行和如下指令得到trace图：

go tool trace -http=127.0.0.1:8000 trace.pprof

如果大家从浏览器上看不到上述图像，首先请更换chrome浏览器，因为目前官方只适配了chrome；如果依旧无法查看改图像，MacOS请按照下述方法进行操作。 1、登录google账号，访问https://developers.chrome.com/origintrials/#/register_trial/2431943798780067841，其中web Origin字段为此后你需要访问的web网址，例如我使用的127.0.0.1:8000。如此你将获得一个Active Token并复制下来。 2、在你的go的安装目录{$GOROOT}/src/cmd/trace/trace.go文件中，找到元素范围，并添加 3、在该目录下分别执行 go build和go install，此后重启Chrome浏览器即可查看上图。

在上图中有几个关键字段，下面进行讲解：

Goroutines：运行中的协程数量；通过点击图中颜色标识可查看相关信息，可以看到在大部分情况下可执行的协程会很多，但是运行中的只有0个或1个，因为我们只用了1核。 Heap：运行中使用的总堆内存；因为此段代码是有内存分配缺陷的，所以heap字段的颜色标识显示堆内存在不断增长中。 Threads：运行中系统进程数量；很显然只有1个。 GC：系统垃圾回收；在程序的末端开始回收资源。 syscalls：系统调用；由上图看到在GC开始只有很微少的一段。 Proc0：系统进程，与使用的处理器的核数有关，1个。

另外我们从图中可以看到程序的总运行时间不到3ms。进一步我们可以进行放大颜色区域，查看详细信息，以下图为例：


可以看到在Proc0轨道上，不同颜色代表不同协程，各个协程都是串行的，执行counter函数的有G7、G8和G9协程，同时Goroutines轨道上的协程数量也相应再减少。伴随着协程的结束，GC也会将内存回收，另外在GC过程中出现了STW（stop the world）过程，这对程序的执行效率会有极大的影响。STW过程会将整个程序通过sleep停止下来，所以在前文中出现的runtime.usleep就是此时由GC调用的。

下面我们使用多个核来运行，只需要改动GOMAXPROCS即可，例如修改成5并获得trace图：

runtime.GOMAXPROCS(5)

从上图可以看到，3个counter协程再0、2、3核上执行，同时程序的运行时间为0.28ms，运行时间大大降低，可见提高cpu核数是可以提高效率的，但是也不是所有场景都适合提高核数，还是需要具体分析。同时为了减少内存的扩容，同样可以预先分配内存，获得trace图如下所示：


由上图看到，由于我们提前分配好足够的内存，系统不需要进行多次扩容，进而进一步减小开销。从slice的源码中看到其实现中包含指针，即其内存是堆内存，而不是C/C++中类似数组的栈空间的分配方式。另外也能看到程序的运行时间为0.18ms，进一步提高运行速度。

另外，trace图还有很多功能，例如查看事件的关联信息等等，通过点击All connected即可生成箭头表示相互关系，大家可以自己探究一下其他功能。


如果我们对counter函数的循环中加上锁会发生什么呢？

func counter(wg *sync.WaitGroup, m *sync.Mutex) {
    wg.Done()

    slice := [100000]int{0}
    c := 1
    for i := 0; i < 100000; i++ {
        mutex.Lock()
        c = i + 1 + 2 + 3 + 4 + 5
        slice[i] = c
        mutex.Unlock()
    }
}
生成trace图如下：


可以看到程序运行的时间又增加了，主要是由于加/放锁使得counter协程的执行时间变长。但是并没有看到不同协程对cpu占有权的切换呀？这是为什么呢？主要是这个协程运行时间太短，而相对而言采样的频率低、粒度大，导致采样数据比较少。如果在程序中人为sleep一段时间，提高采样数量就可以真实反映cpu占有权的切换。例如在main函数中sleep 1秒则出现下图所示的trace图：


如果对go协程加锁呢？

for i := 0; i < 3; i ++ {
        mutex.Lock()
        go counter(&wg)
        time.Sleep(time.Millisecond)
        mutex.Unlock()
    }
从得到的trace图可以看出，其cpu主要时间都是在睡眠等待中，所以在程序中应该减少此类sleep操作。


trace图可以非常完整的跟踪程序的整个执行周期，所以大家可以从整体到局部分析优化程序。我们可以先使用pprof完成初步的检查和优化，主要是CPU和内存，而trace主要是用于各个协程的执行关系的分析，从而优化结构。

本文主要讲解了一些性能评测和trace的方法，仍然比较浅显，更多用法大家可以自己去探索。

## 参考文献
- [Go性能优化](https://studygolang.com/articles/23047)
- [golang系列—性能评测之pprof+火焰图+trace](https://zhuanlan.zhihu.com/p/141640004)
- [Go语言之pprof的性能调优”燥起来“](https://zhuanlan.zhihu.com/p/301065345)
