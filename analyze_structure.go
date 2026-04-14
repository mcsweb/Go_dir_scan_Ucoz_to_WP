package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	rootPath           = "."
	maxVisibleFiles    = 3  // Сколько файлов показывать перед скрытием (для не-картинок)
	maxVisibleImages   = 2  // Сколько картинок показывать перед скрытием
)

// Расширения, которые считаем "шумом" (массовые файлы)
var imageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".bmp":  true,
	".webp": true,
}

type Stats struct {
	TotalDirs      int
	TotalFiles     int
	TotalImages    int
	TotalContent   int // Файлы, которые не картинки
	TotalSize      int64
	Extensions     map[string]int
	ImportantFiles []string // Список важных файлов (не картинки) для быстрого просмотра
}

func main() {
	fmt.Println("=== Глубокий анализ структуры Ucoz архива ===")
	fmt.Println("(Показываем все папки, скрываем множественные JPG/PNG)")
	
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		fmt.Printf("Ошибка: Путь '%s' не найден.\n", rootPath)
		return
	}

	stats := &Stats{
		Extensions: make(map[string]int),
	}

	fmt.Printf("\nСканирование: %s\n", filepath.ToSlash(rootPath))
	fmt.Println("--------------------------------------------------")
	
	err := scanDirectory(rootPath, 0, stats)
	if err != nil {
		fmt.Printf("Ошибка при сканировании: %v\n", err)
		return
	}

	printStats(stats)
}

func scanDirectory(path string, depth int, stats *Stats) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsPermission(err) {
			return nil
		}
		return err
	}

	var dirs []os.DirEntry
	var files []os.DirEntry

	// Разделяем папки и файлы
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	// Сортируем для стабильного вывода
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	indent := strings.Repeat("  ", depth)
	
	// Вывод имени текущей папки
	dirName := filepath.Base(path)
	if depth == 0 {
		dirName = dirName + " (Корень)"
	}
	fmt.Printf("%s📁 %s/\n", indent, dirName)

	// 1. Рекурсивно обрабатываем ВСЕ папки (без ограничений по количеству)
	for _, d := range dirs {
		nextPath := filepath.Join(path, d.Name())
		err := scanDirectory(nextPath, depth+1, stats)
		if err != nil {
			fmt.Printf("%s  [Ошибка доступа: %v]\n", indent, err)
		}
	}

	// 2. Обрабатываем файлы
	// Разделяем на картинки и остальное
	var contentFiles []os.DirEntry
	var imageFiles []os.DirEntry

	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if imageExts[ext] {
			imageFiles = append(imageFiles, f)
		} else {
			contentFiles = append(contentFiles, f)
		}
	}

	// --- Вывод контентных файлов (HTML, TXT, PHP и т.д.) ---
	displayedContent := 0
	for _, f := range contentFiles {
		info, err := f.Info()
		size := int64(0)
		if err == nil {
			size = info.Size()
			stats.TotalFiles++
			stats.TotalContent++
			stats.TotalSize += size
			
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if ext == "" { ext = "[no_ext]" }
			stats.Extensions[ext]++
			
			// Сохраняем важные файлы для статистики
			if stats.TotalContent <= 20 && ext != ".go" {
				stats.ImportantFiles = append(stats.ImportantFiles, filepath.Join(path, f.Name()))
			}
		}

		if displayedContent < maxVisibleFiles {
			fmt.Printf("%s  📄 %s (%s)\n", indent, f.Name(), formatSize(size))
			displayedContent++
		}
	}
	if len(contentFiles) > maxVisibleFiles {
		fmt.Printf("%s  ... и ещё %d файл(ов) (текст/код)\n", indent, len(contentFiles)-maxVisibleFiles)
	}

	// --- Вывод файлов изображений (сокращенно) ---
	displayedImages := 0
	for _, f := range imageFiles {
		info, err := f.Info()
		size := int64(0)
		if err == nil {
			size = info.Size()
			stats.TotalFiles++
			stats.TotalImages++
			stats.TotalSize += size
			stats.Extensions[strings.ToLower(filepath.Ext(info.Name()))]++
		}

		if displayedImages < maxVisibleImages {
			fmt.Printf("%s  🖼️  %s (%s)\n", indent, f.Name(), formatSize(size))
			displayedImages++
		}
	}
	if len(imageFiles) > maxVisibleImages {
		fmt.Printf("%s  ... и ещё %d изображений\n", indent, len(imageFiles)-maxVisibleImages)
	}
	
	stats.TotalDirs++
	return nil
}

func printStats(s *Stats) {
	fmt.Println("\n==================================================")
	fmt.Println("ИТОГОВАЯ СТАТИСТИКА:")
	fmt.Printf("Всего папок:       %d\n", s.TotalDirs)
	fmt.Printf("Всего файлов:      %d\n", s.TotalFiles)
	fmt.Printf("  - Изображения:   %d\n", s.TotalImages)
	fmt.Printf("  - Контент:       %d\n", s.TotalContent)
	fmt.Printf("Общий размер:      %s\n", formatSize(s.TotalSize))
	
	fmt.Println("\nТоп расширений (кроме картинок):")
	type kv struct {
		Key   string
		Value int
	}
	var sortedExt []kv
	for k, v := range s.Extensions {
		if imageExts[k] { continue } // Пропускаем картинки в топе
		sortedExt = append(sortedExt, kv{k, v})
	}
	sort.Slice(sortedExt, func(i, j int) bool {
		return sortedExt[i].Value > sortedExt[j].Value
	})

	count := 0
	for _, e := range sortedExt {
		if count >= 15 { break }
		fmt.Printf("  %-10s : %d шт.\n", e.Key, e.Value)
		count++
	}

	if len(s.ImportantFiles) > 0 {
		fmt.Println("\nПримеры найденных контентных файлов:")
		for _, f := range s.ImportantFiles {
			fmt.Printf("  - %s\n", f)
		}
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
