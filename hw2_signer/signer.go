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

// считает значение crc32(data)+"~"+crc32(md5(data)) ( конкатенация двух строк через ~), где data - то что пришло на вход (по сути - числа из первой функции)
func SingleHash(in, out chan interface{}) {
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
		send := DataSignerCrc32(data) + "~" + DataSignerCrc32(DataSignerMd5(data))
		log.Printf("SingleHash for %s: %s", data, send)
		out <- send
	}
}

// MultiHash считает значение crc32(th+data)) (конкатенация цифры, приведённой к строке и строки), где th=0..5 ( т.е. 6 хешей на каждое входящее значение )
func MultiHash(in, out chan interface{}) {
	results := make([]string, 6)
	for value := range in {
		data, ok := value.(string)
		if !ok {
			panic("Non string data sent")
		}
		for i := 0; i < 6; i++ {
			results[i] = DataSignerCrc32(fmt.Sprintf("%1d%s", i, data))
		}
		//  потом берёт конкатенацию результатов в порядке расчета (0..5), где data - то что пришло на вход (и ушло на выход из SingleHash)
		joined := strings.Join(results, "")
		log.Printf("%s:MultiHash result: %s", data, joined)
		out <- joined
	}
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
