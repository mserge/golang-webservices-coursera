package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
)

// сюда писать код

func ExecuteJob(wg *sync.WaitGroup, worker job, first bool, in, out chan interface{}) {
	//log.Printf("Start %v with channel %v %v ", worker, in, out)
	worker(in, out)
	//log.Printf("Finish %v with channel %v %v ", worker, in, out)
	if first {
		close(in) //close for first only
	}
	close(out) // close as soon it finishes and it will close for next job
	wg.Done()
}

func ExecutePipeline(jobs ...job) {
	ch := make(chan interface{}, 100)
	wg := &sync.WaitGroup{} // wait_2.go инициализируем группу
	for i, pipe_job := range jobs {
		ch_out := make(chan interface{}, 100)
		wg.Add(1) // wait_2.go добавляем воркер
		go ExecuteJob(wg, pipe_job, i == 0, ch, ch_out)
		ch = ch_out
	}
	wg.Wait() //  пока wg.Done() не приведёт счетчик к 0
}

type CRC32result struct {
	id    int
	value string
}

func DataSignerCrc32Job(id int, data string, out chan CRC32result, group *sync.WaitGroup) {
	defer group.Done()

	out <- CRC32result{id, DataSignerCrc32(data)}
	return
}

func RunDataSigner(separator string, datain ...string) string {
	wg := &sync.WaitGroup{} // инициализируем группу
	resultsCh := make(chan CRC32result, len(datain))

	for i, data := range datain {
		wg.Add(1)
		go DataSignerCrc32Job(i, data, resultsCh, wg)
	}
	wg.Wait()
	dataout := make([]string, len(datain))
	for _ = range dataout {
		result := <-resultsCh
		dataout[result.id] = result.value
	}
	return strings.Join(dataout, separator)
}

// считает значение crc32(data)+"~"+crc32(md5(data)) ( конкатенация двух строк через ~), где data - то что пришло на вход (по сути - числа из первой функции)
func SingleHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{} // инициализируем группу
	for value := range in {
		data := ""
		switch value.(type) {
		case string:
			data = value.(string)
		case int:
			data = fmt.Sprintf("%d", value.(int))
		default:
			panic("Non string data sent")
		}
		// TODO - limit call?
		md5 := DataSignerMd5(data)
		log.Printf("MD5 for %s: %s", data, md5)
		wg.Add(1)
		go runSingle(data, md5, out, wg)
	}
	wg.Wait()
}

func runSingle(data string, md5 string, out chan interface{}, group *sync.WaitGroup) {
	defer group.Done()
	send := RunDataSigner("~", data, md5)
	log.Printf("SingleHash for %s: %s", data, send)
	out <- send
}

// MultiHash считает значение crc32(th+data)) (конкатенация цифры, приведённой к строке и строки), где th=0..5 ( т.е. 6 хешей на каждое входящее значение )
func MultiHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{} // инициализируем группу
	results := make([]string, 6)
	for value := range in {
		data, ok := value.(string)
		if !ok {
			panic("Non string data sent")
		}
		for i := 0; i < 6; i++ {
			results[i] = fmt.Sprintf("%1d%s", i, data)
		}
		wg.Add(1)
		go runMultiple(results, data, out, wg)
	}
	wg.Wait()
}

func runMultiple(results []string, data string, out chan interface{}, group *sync.WaitGroup) {
	defer group.Done()
	//  потом берёт конкатенацию результатов в порядке расчета (0..5), где data - то что пришло на вход (и ушло на выход из SingleHash)
	joined := RunDataSigner("", results...)
	log.Printf("%s:MultiHash result: %s", data, joined)
	out <- joined
}

// CombineResults получает все результаты, сортирует (https://golang.org/pkg/sort/), объединяет отсортированный результат через _ (символ подчеркивания) в одну строку
func CombineResults(in, out chan interface{}) {
	results := make([]string, 0, 100)
	for data_raw := range in {
		data, ok := (data_raw).(string)
		if !ok {
			panic("Non string data sent")
		}
		results = append(results, data)
	}
	sort.Strings(results)
	joined := strings.Join(results, "_")
	out <- joined
}
