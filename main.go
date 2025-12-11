package main

// #include <stdlib.h>
import "C"
import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

const (
	PAGE_SIZE      = 4096              // 4 KiB
	PAUSE_INTERVAL = 128 * 1024 * 1024 // 128 MiB
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Использование: program <r|w> <размер_в_MiB>")
		fmt.Println("  r - режим чтения")
		fmt.Println("  w - режим записи")
		os.Exit(1)
	}

	mode := os.Args[1]
	if mode != "r" && mode != "w" {
		fmt.Println("Ошибка: режим должен быть 'r' или 'w'")
		os.Exit(1)
	}

	sizeMB, err := strconv.Atoi(os.Args[2])
	if err != nil || sizeMB <= 0 {
		fmt.Println("Ошибка: размер должен быть положительным числом")
		os.Exit(1)
	}

	size := C.size_t(sizeMB * 1024 * 1024)
	pageSize := os.Getpagesize()
	if pageSize <= 0 {
		pageSize = PAGE_SIZE
	}

	fmt.Printf("Режим:          %s (%s)\n", mode, getModeDescription(mode))
	fmt.Printf("Размер:         %d MiB (%d байт)\n", sizeMB, size)
	fmt.Printf("Размер страницы: %d байт\n", pageSize)
	fmt.Printf("Интервал паузы: %d MiB\n", PAUSE_INTERVAL/(1024*1024))

	ptr := C.malloc(size)
	if ptr == nil {
		panic("malloc failed: не удалось выделить память")
	}
	defer C.free(ptr)

	buffer := (*[1 << 31]byte)(ptr)[:size:size]

	nonZeroCount := 0
	for i := 0; i < min(1024, int(size)); i++ {
		if buffer[i] != 0 {
			nonZeroCount++
		}
	}
	fmt.Printf("Ненулевых байт в первом KiB: %d из 1024\n", nonZeroCount)

	printMemStats()

	fmt.Printf("\nНачинаем обработку памяти в режиме '%s'...\n", mode)
	waitForEnter()

	start := time.Now()
	processedBytes := 0
	checksum := uint64(0)

	reader := bufio.NewReader(os.Stdin)

	for i := 0; i < int(size); i += pageSize {
		if mode == "w" {
			buffer[i] = 0x42
		} else {
			checksum += uint64(buffer[i])
		}

		processedBytes += pageSize

		if processedBytes%PAUSE_INTERVAL == 0 {
			elapsed := time.Since(start)
			mbProcessed := processedBytes / (1024 * 1024)

			fmt.Printf("Обработано: %d MiB из %d MiB (%.1f%%)\n",
				mbProcessed, sizeMB, float64(mbProcessed)/float64(sizeMB)*100)
			fmt.Printf("Время: %v\n", elapsed.Round(time.Millisecond))
			printMemStats()
			fmt.Printf("нажмите энтер для продолжения \n")

			reader.ReadBytes('\n')
		}
	}

	elapsed := time.Since(start)

	fmt.Printf("\n Обработка завершена\n")
	fmt.Printf("Общее время:     %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Обработано:      %d MiB\n", processedBytes/(1024*1024))
	fmt.Printf("Скорость:        %.2f MiB/сек\n",
		float64(processedBytes)/(1024*1024)/elapsed.Seconds())

	if mode == "r" {
		fmt.Printf("Контрольная сумма: %d\n", checksum)
	}

	printMemStats()

	waitForEnter()
}

func getModeDescription(mode string) string {
	if mode == "w" {
		return "запись на каждой странице"
	}
	return "чтение с каждой страницы"
}

func printMemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("Статистика памяти:\n")
	fmt.Printf("  Go Alloc:  %6d MiB (память выделенная Go runtime)\n",
		m.Alloc/(1024*1024))
	fmt.Printf("  Go Sys:    %6d MiB (память от ОС для Go)\n",
		m.Sys/(1024*1024))
	fmt.Printf("  NumGC:     %6d     (количество сборок мусора)\n", m.NumGC)
}

func waitForEnter() {
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
