package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// Путь к архиву. Если скрипт лежит рядом с папкой архива, оставьте "." или укажите имя папки
	// Например: "./ucoz_archive" или "C:/Users/User/Desktop/ucoz_backup"
	rootPath = "." 
	// Максимальное количество файлов/папок для показа в одной директории
	maxItemsPerDir = 10
)

type Stats struct {
	TotalDirs  int
	TotalFiles int
	TotalSize  int64
	Extensions map[string]int
}

func main() {
	fmt.Println("=== Анализ структуры Ucoz архива ===")
	
	// Проверка существования пути
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		fmt.Printf("Ошибка: Путь '%s' не найден.\n", rootPath)
		fmt.Println("Пожалуйста, распакуйте архив в эту папку или измените переменную rootPath в коде.")
		return
	}

	stats := &Stats{
		Extensions: make(map[string]int),
	}

	fmt.Printf("\nСканирование начиная с: %s\n", filepath.ToSlash(rootPath))
	fmt.Println("--------------------------------------------------")
	
	// Запуск рекурсивного обхода
	err := scanDirectory(rootPath, 0, stats)
	if err != nil {
		fmt.Printf("Ошибка при сканировании: %v\n", err)
		return
	}

	printStats(stats)
}

func scanDirectory(path string, depth int, stats *Stats) error {
	// Ограничение глубины, если нужно (сейчас без ограничений по глубине)
	
	entries, err := os.ReadDir(path)
	if err != nil {
		// Игнорируем ошибки доступа к некоторым системным папкам, но выводим предупреждение
		if os.IsPermission(err) {
			return nil
		}
		return err
	}

	// Разделяем папки и файлы для сортировки
	var dirs []os.DirEntry
	var files []os.DirEntry

	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	// Сортируем по имени для стабильного вывода
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	// --- Печать текущей директории ---
	indent := strings.Repeat("  ", depth)
	
	// Показываем имя текущей папки (для корня можно не показывать или показать особым образом)
	if depth > 0 {
		fmt.Printf("%s📁 %s/\n", indent, filepath.Base(path))
	} else {
		fmt.Printf("📂 %s (Корень)\n", filepath.Base(path))
	}

	// Обработка папок
	displayedDirs := 0
	for _, d := range dirs {
		if displayedDirs >= maxItemsPerDir {
			remaining := len(dirs) - displayedDirs
			fmt.Printf("%s  ... и ещё %d папок(-ки)\n", indent, remaining)
			break
		}
		
		// Рекурсивный вызов
		nextPath := filepath.Join(path, d.Name())
		err := scanDirectory(nextPath, depth+1, stats)
		if err != nil {
			fmt.Printf("%s  Ошибка доступа к папке: %v\n", indent, err)
		}
		displayedDirs++
	}

	// Обработка файлов
	displayedFiles := 0
	for _, f := range files {
		if displayedFiles >= maxItemsPerDir {
			remaining := len(files) - displayedFiles
			fmt.Printf("%s  ... и ещё %d файл(ов)\n", indent, remaining)
			break
		}

		info, err := f.Info()
		size := int64(0)
		if err == nil {
			size = info.Size()
			stats.TotalFiles++
			stats.TotalSize += size
			
			// Сбор статистики по расширениям
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if ext == "" {
				ext = "[без расширения]"
			}
			stats.Extensions[ext]++
		}

		sizeStr := formatSize(size)
		fmt.Printf("%s  📄 %s (%s)\n", indent, f.Name(), sizeStr)
		displayedFiles++
	}
	
	stats.TotalDirs++
	return nil
}

func printStats(s *Stats) {
	fmt.Println("\n==================================================")
	fmt.Println("ИТОГОВАЯ СТАТИСТИКА:")
	fmt.Printf("Всего папок:     %d\n", s.TotalDirs)
	fmt.Printf("Всего файлов:    %d\n", s.TotalFiles)
	fmt.Printf("Общий размер:    %s\n", formatSize(s.TotalSize))
	
	fmt.Println("\nТоп расширений файлов:")
	
	// Сортировка расширений по количеству
	type kv struct {
		Key   string
		Value int
	}
	var sortedExt []kv
	for k, v := range s.Extensions {
		sortedExt = append(sortedExt, kv{k, v})
	}
	sort.Slice(sortedExt, func(i, j int) bool {
		return sortedExt[i].Value > sortedExt[j].Value
	})

	count := 0
	for _, e := range sortedExt {
		if count >= 15 { // Показываем топ-15
			break
		}
		fmt.Printf("  %-10s : %d шт.\n", e.Key, e.Value)
		count++
	}
	if len(sortedExt) > 15 {
		fmt.Println("  ... и другие")
	}
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
