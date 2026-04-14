package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Структуры данных
type Category struct {
	ID       int
	ParentID int
	Name     string
	Count    int
	Children []int
}

type Author struct {
	Login string
	Name  string
	Count int
}

type Stats struct {
	Categories map[int]*Category
	Authors    map[string]*Author
	Years      map[string]int
	TagsCount  int
	ImagesCount int
}

func main() {
	fmt.Println("=== Детальный анализ контента Ucoz (Исправленный парсинг) ===")
	
	stats := &Stats{
		Categories: make(map[int]*Category),
		Authors:    make(map[string]*Author),
		Years:      make(map[string]int),
	}

	// 1. Анализ рубрик Новостей
	fmt.Println("\n[1] Анализ рубрик Новостей (nw_nw.txt)...")
	parseCategories("_s1/nw_nw.txt", stats, "news")

	// 2. Анализ рубрик Публикаций (inf.json)
	fmt.Println("[2] Анализ рубрик Публикаций (inf.json)...")
	parsePublCategories("_s1/inf.json", stats)

	// 3. Анализ авторов
	fmt.Println("[3] Анализ авторов (users.txt / ugen.txt)...")
	parseAuthors("_s1/users.txt", "_s1/ugen.txt", stats)

	// 4. Подсчет материалов и распределение
	fmt.Println("[4] Обработка новостей (news.txt)...")
	countItems("_s1/news.txt", stats, "news")

	fmt.Println("[5] Обработка публикаций (publ.txt)...")
	countItems("_s1/publ.txt", stats, "publ")

	// 5. Теги и изображения
	fmt.Println("[6] Подсчет тегов и изображений...")
	countTags("_s1/tags.txt", stats)
	countImages(".", stats)

	// ВЫВОД ОТЧЕТА
	printReport(stats)
}

// Парсинг категорий из pipe-файла
func parseCategories(path string, stats *Stats, source string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("   ⚠️ Файл %s не найден\n", path)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}

		id, _ := strconv.Atoi(parts[0])
		parentID, _ := strconv.Atoi(parts[1])
		// name может быть в parts[4] или parts[3] в зависимости от версии экспорта
		name := strings.TrimSpace(parts[4])
		if name == "" && len(parts) > 3 {
			name = strings.TrimSpace(parts[3])
		}
		
		// Пропускаем мусор
		if strings.Contains(name, "`") || strings.Contains(name, "jpg") {
			continue
		}

		if _, exists := stats.Categories[id]; !exists {
			stats.Categories[id] = &Category{ID: id, ParentID: parentID, Name: name, Children: []int{}}
		} else {
			stats.Categories[id].Name = name
			stats.Categories[id].ParentID = parentID
		}

		if parentID > 0 {
			if _, exists := stats.Categories[parentID]; exists {
				stats.Categories[parentID].Children = append(stats.Categories[parentID].Children, id)
			}
		}
	}
}

// Парсинг категорий публикаций из JSON
func parsePublCategories(path string, stats *Stats) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("   ⚠ Файл %s не найден\n", path)
		return
	}

	var publMap map[string][]string
	if err := json.Unmarshal(data, &publMap); err != nil {
		fmt.Printf("   ⚠ Ошибка JSON: %v\n", err)
		return
	}

	for idStr, vals := range publMap {
		id, _ := strconv.Atoi(idStr)
		name := ""
		parentID := 0
		
		if len(vals) >= 5 {
			name = vals[0] // Обычно первое поле - имя
			// В inf.json структура может отличаться, проверим наличие родителя
			if len(vals) > 1 {
				if pId, err := strconv.Atoi(vals[1]); err == nil {
					parentID = pId
				}
			}
		}

		if name != "" && !strings.Contains(name, "`") {
			if _, exists := stats.Categories[id]; !exists {
				stats.Categories[id] = &Category{ID: id, ParentID: parentID, Name: name, Children: []int{}}
			} else {
				stats.Categories[id].Name = name
				stats.Categories[id].ParentID = parentID
			}
			
			if parentID > 0 {
				if _, exists := stats.Categories[parentID]; exists {
					stats.Categories[parentID].Children = append(stats.Categories[parentID].Children, id)
				}
			}
		}
	}
}

// Парсинг авторов
func parseAuthors(usersPath, ugenPath string, stats *Stats) {
	// Читаем users.txt для логинов
	usersFile, err := os.Open(usersPath)
	if err == nil {
		defer usersFile.Close()
		scanner := bufio.NewScanner(usersFile)
		for scanner.Scan() {
			parts := strings.Split(scanner.Text(), "|")
			if len(parts) >= 1 {
				login := strings.TrimSpace(parts[0])
				if login != "" {
					if _, exists := stats.Authors[login]; !exists {
						stats.Authors[login] = &Author{Login: login, Name: login}
					}
				}
			}
		}
	}

	// Читаем ugen.txt для имен
	ugenFile, err := os.Open(ugenPath)
	if err == nil {
		defer ugenFile.Close()
		scanner := bufio.NewScanner(ugenFile)
		for scanner.Scan() {
			parts := strings.Split(scanner.Text(), "|")
			if len(parts) >= 2 {
				login := strings.TrimSpace(parts[0])
				name := strings.TrimSpace(parts[1])
				if author, exists := stats.Authors[login]; exists {
					if name != "" && name != login {
						author.Name = name
					}
				} else {
					stats.Authors[login] = &Author{Login: login, Name: name}
				}
			}
		}
	}
}

// Подсчет элементов (новости/публикации)
func countItems(path string, stats *Stats, itemType string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Разделяем только первые 10-15 элементов, так как текст может содержать |
		parts := strings.SplitN(line, "|", 15)
		if len(parts) < 12 {
			continue
		}

		// Определяем категорию (обычно поле 2 или 3)
		// Для news.txt: ID|CatID|SubCatID|Year|Month|Day...
		// Для publ.txt: ID|CatID|SubCatID|Date...
		
		catIDStr := parts[1] 
		catID, _ := strconv.Atoi(catIDStr)
		
		// Проверка на валидность категории (чтобы не попасть в мусор)
		if cat, exists := stats.Categories[catID]; exists && cat.Name != "" && !strings.Contains(cat.Name, "`") {
			cat.Count++
		} else if catID > 0 && catID < 1000 { // Эвристика для новых категорий
             // Если категория не загружена заранее (например из inf.json), создадим заглушку
             if _, exists := stats.Categories[catID]; !exists {
                 stats.Categories[catID] = &Category{ID: catID, Name: fmt.Sprintf("Неизвестная #%d", catID)}
                 stats.Categories[catID].Count++
             }
		}

		// Определение года
		year := "Unknown"
		if itemType == "news" && len(parts) >= 7 {
			// news.txt: ...|Year|Month|Day|...
			yStr := parts[3]
			if y, err := strconv.Atoi(yStr); err == nil && y > 2000 && y < 2030 {
				year = yStr
			}
		} else if itemType == "publ" {
			// Publ часто имеет дату в другом формате или таймстамп
			// Попробуем найти год в первых полях
			for i := 3; i < len(parts) && i < 8; i++ {
				if y, err := strconv.Atoi(parts[i]); err == nil && y > 2000 && y < 2030 {
					year = parts[i]
					break
				}
			}
			// Если не нашли явный год, проверяем таймстамп (последние поля иногда)
			if year == "Unknown" && len(parts) > 10 {
				if ts, err := strconv.Atoi(parts[5]); err == nil && ts > 1000000000 {
					t := time.Unix(int64(ts), 0)
					year = strconv.Itoa(t.Year())
				}
			}
		}
		
		if year != "Unknown" {
			stats.Years[year]++
		} else {
			stats.Years["Unknown"]++
		}

		// Автор (обычно поле после даты или в конце, зависит от типа)
		// В news.txt автор часто в поле 10 или 11
		if len(parts) > 10 {
			authorLogin := strings.TrimSpace(parts[10])
			if authorLogin != "" && !strings.Contains(authorLogin, " ") {
				if _, exists := stats.Authors[authorLogin]; !exists {
					stats.Authors[authorLogin] = &Author{Login: authorLogin, Name: authorLogin}
				}
				stats.Authors[authorLogin].Count++
			}
		}
	}
}

func countTags(path string, stats *Stats) {
	if _, err := os.Stat(path); err == nil {
		file, _ := os.Open(path)
		defer file.Close()
		scanner := bufio.NewScanner(file)
		count := 0
		for scanner.Scan() {
			count++
		}
		stats.TagsCount = count
	}
}

func countImages(root string, stats *Stats) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
				stats.ImagesCount++
			}
		}
		return nil
	})
}

func printReport(stats *Stats) {
	fmt.Println("\n==================================================")
	fmt.Println("           ОТЧЕТ ПО МИГРАЦИИ (ИСПРАВЛЕННЫЙ)")
	fmt.Println("==================================================")

	fmt.Println("\n📂 СТРУКТУРА РУБРИК (Только валидные)")
	
	// Сортируем категории: сначала корневые (ParentID=0), потом вложенные
	var rootCats []*Category
	for _, c := range stats.Categories {
		if c.ParentID == 0 && c.Count > 0 && c.Name != "" && !strings.Contains(c.Name, "`") {
			rootCats = append(rootCats, c)
		}
	}
	sort.Slice(rootCats, func(i, j int) bool { return rootCats[i].Count > rootCats[j].Count })

	for _, cat := range rootCats {
		fmt.Printf("📁 %s (ID:%d) [%d мат.]\n", cat.Name, cat.ID, cat.Count)
		// Выводим детей
		for _, childID := range cat.Children {
			if child, exists := stats.Categories[childID]; exists && child.Count > 0 {
				fmt.Printf("  └── %s (ID:%d) [%d мат.]\n", child.Name, child.ID, child.Count)
			}
		}
	}

	fmt.Println("\n📊 СТАТИСТИКА МАТЕРИАЛОВ")
	totalNews := 0
	totalPubl := 0
	// Суммируем вручную, так как в функции countItems мы не разделяли итоговые суммы явно, но можем оценить по годам
	// Для простоты выведем топ лет
	fmt.Println("Распределение по годам (Все материалы):")
	type kv struct { Key string; Value int }
	var sortedYears []kv
	for k, v := range stats.Years {
		sortedYears = append(sortedYears, kv{k, v})
	}
	sort.Slice(sortedYears, func(i, j int) bool { 
		if sortedYears[i].Key == "Unknown" { return false }
		if sortedYears[j].Key == "Unknown" { return true }
		return sortedYears[i].Key > sortedYears[j].Key 
	})

	count := 0
	for _, y := range sortedYears {
		if count >= 15 { break }
		fmt.Printf("  %s: %d\n", y.Key, y.Value)
		count++
	}

	fmt.Println("\n👥 ТОП-10 АВТОРОВ")
	var authorsList []*Author
	for _, a := range stats.Authors {
		authorsList = append(authorsList, a)
	}
	sort.Slice(authorsList, func(i, j int) bool { return authorsList[i].Count > authorsList[j].Count })

	for i, a := range authorsList {
		if i >= 10 { break }
		fmt.Printf("  %d. %s (%s) - %d статей\n", i+1, a.Name, a.Login, a.Count)
	}

	fmt.Printf("\n🖼️  Изображений всего: %d\n", stats.ImagesCount)
	fmt.Printf("🏷️  Записей тегов: %d\n", stats.TagsCount)

	fmt.Println("\n💡 РЕКОМЕНДАЦИИ:")
	fmt.Println("1. Создать в WP рубрики, указанные выше (иерархия сохранена).")
	fmt.Println("2. Новости и Публикации сливать в одну ленту 'Post', различая рубриками.")
	fmt.Println("3. Авторов создавать по мере импорта, сохраняя логин Ucoz в мета-поле для связи.")
}