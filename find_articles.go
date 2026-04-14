package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("=== Поиск контента статей Ucoz ===")
	fmt.Println("Сканирование текстовых файлов и анализ структуры...")
	fmt.Println("--------------------------------------------------")

	root := "."
	
	// 1. Проверяем важные директории Ucoz на наличие текстовых данных
	dirsToCheck := []string{"_s1", "_sh", "_bd", "_dr", "_fr"}
	
	foundContent := false
	
	for _, dir := range dirsToCheck {
		fullPath := filepath.Join(root, dir)
		if _, err := os.Stat(fullPath); err == nil {
			fmt.Printf("\n[Анализ директории: %s]\n", dir)
			err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					return nil
				}
				
				ext := strings.ToLower(filepath.Ext(path))
				name := info.Name()
				
				// Ищем потенциальные файлы с данными
				if ext == ".txt" || ext == ".xml" || ext == ".json" || ext == ".csv" || 
				   ext == ".upf" || ext == ".upp" || strings.Contains(name, "blog") || 
				   strings.Contains(name, "news") || strings.Contains(name, "post") {
					
					fmt.Printf("  📄 Найден файл: %s (%s)\n", path, formatSize(info.Size()))
					
					// Пробуем прочитать первые строки, чтобы понять суть
					if ext == ".txt" || ext == ".xml" || ext == ".json" || ext == ".csv" {
						printFilePreview(path, 5)
						foundContent = true
					} else if ext == ".upf" || ext == ".upp" {
						fmt.Println("     ⚠️ Бинарный/Специфичный формат Ucoz (.upf/.upp). Требуется спец. парсер или распаковка основного архива.")
					}
				}
				return nil
			})
			if err != nil {
				fmt.Printf("Ошибка сканирования %s: %v\n", dir, err)
			}
		} else {
			fmt.Printf("Директория %s не найдена.\n", dir)
		}
	}

	// 2. Ищем HTML файлы, которые могут содержать статьи
	fmt.Println("\n[Поиск HTML файлов в корне и подпапках...]")
	htmlCount := 0
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".html" {
			// Исключаем системные файлы
			if !strings.Contains(path, ".well-known") {
				if htmlCount < 10 {
					fmt.Printf("  🌐 %s (%s)\n", path, formatSize(info.Size()))
					printFilePreview(path, 3)
				}
				htmlCount++
			}
		}
		return nil
	})
	if htmlCount > 10 {
		fmt.Printf("  ... и ещё %d HTML файлов\n", htmlCount-10)
	}

	// 3. Проверка большого архива
	bkArchive := ""
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if strings.HasPrefix(info.Name(), "_bk_") && strings.HasSuffix(info.Name(), ".zip") {
			bkArchive = path
		}
		return nil
	})

	fmt.Println("\n==================================================")
	fmt.Println("ВЫВОДЫ:")
	
	if !foundContent && htmlCount == 0 {
		fmt.Println("❌ Тексты статей НЕ найдены в открытых файлах.")
		fmt.Println("   Вероятно, весь контент (тексты, рубрики, метки) находится внутри архива базы данных.")
		
		if bkArchive != "" {
			fmt.Printf("\n🔴 ВНИМАНИЕ: Обнаружен архив базы данных: %s (%s)\n", bkArchive, formatSize(getFileSize(bkArchive)))
			fmt.Println("   Для миграции необходимо:")
			fmt.Println("   1. Распаковать этот архив в отдельную папку (например, ucoz_dump).")
			fmt.Println("      * Внутри может быть SQL дамп или файлы .udata/.xml")
			fmt.Println("   2. Запустить скрипт снова, указав путь к распакованной папке.")
		} else {
			fmt.Println("   Архив базы данных (_bk_*.zip) не найден. Проверьте наличие полного экспорта Ucoz.")
		}
	} else {
		fmt.Println("✅ Найдены потенциальные файлы с контентом. Анализ выше.")
	}
	
	fmt.Println("\n💡 Подсказка: Структура картинок (_nw, _ph) готова к переносу. Главное - найти тексты.")
}

func printFilePreview(path string, lines int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	fmt.Print("     ")
	for i := 0; i < lines; i++ {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			break
		}
		line = strings.TrimSpace(line)
		if len(line) > 80 {
			line = line[:80] + "..."
		}
		fmt.Printf("%s\n     ", line)
		if err == io.EOF {
			break
		}
	}
	fmt.Println()
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}