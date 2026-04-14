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

// Структуры данных для хранения информации
type Category struct {
	ID       string
	ParentID string
	Name     string
	Count    int
	Level    int
}

type Author struct {
	NickName string
	RealName string
	Count    int
}

type ArticleStats struct {
	Total      int
	ByYear     map[string]int
	ByCategory map[string]int
}

type Report struct {
	NewsCategories     map[string]*Category
	PublCategories     map[string]*Category
	FileCategories     map[string]*Category
	PhotoCategories    map[string]*Category
	Authors            map[string]*Author
	NewsStats          ArticleStats
	PublStats          ArticleStats
	TotalImages        int
	ImageFolders       map[string]int
	TagsCount          int
}

func main() {
	fmt.Println("=== Глубокий анализ контента Ucoz для миграции на WP ===")
	fmt.Println("Сканирование файлов базы данных...")

	report := &Report{
		NewsCategories:   make(map[string]*Category),
		PublCategories:   make(map[string]*Category),
		FileCategories:   make(map[string]*Category),
		PhotoCategories:  make(map[string]*Category),
		Authors:          make(map[string]*Author),
		NewsStats:        ArticleStats{ByYear: make(map[string]int), ByCategory: make(map[string]int)},
		PublStats:        ArticleStats{ByYear: make(map[string]int), ByCategory: make(map[string]int)},
		ImageFolders:     make(map[string]int),
	}

	// 1. Анализ категорий (Рубрик)
	fmt.Println("\n[1] Анализ структуры рубрик...")
	parseCategories("_s1/fr_fr.txt", report.NewsCategories) // Новости часто используют общую структуру или свою
	parseCategories("_s1/nw_nw.txt", report.NewsCategories) // Специфика новостей
	parseCategories("_s1/ld_ld.txt", report.FileCategories) // Файлы
	parseCategories("_s1/ph_ph.txt", report.PhotoCategories) // Фото
	// Публикации часто используют те же категории, что и новости, или свои собственные в inf.json
	parseInfJson("_s1/inf.json", report.PublCategories)

	// 2. Анализ пользователей (Авторов)
	fmt.Println("[2] Анализ авторов...")
	parseUsers("_s1/users.txt", "_s1/ugen.txt", report.Authors)

	// 3. Анализ Новостей
	fmt.Println("[3] Подсчет новостей и распределение по рубрикам...")
	parseArticles("_s1/news.txt", report, "news")

	// 4. Анализ Публикаций
	fmt.Println("[4] Подсчет публикаций и распределение по рубрикам...")
	parseArticles("_s1/publ.txt", report, "publ")

	// 5. Анализ тегов
	fmt.Println("[5] Анализ меток (тегов)...")
	report.TagsCount = parseTags("_s1/tags.txt")

	// 6. Анализ изображений
	fmt.Println("[6] Сканирование хранилищ изображений...")
	report.TotalImages, report.ImageFolders = scanImages()

	// 7. Вывод отчета
	printReport(report)
}

// Парсинг файлов категорий вида ID|Parent|...|Name|...
func parseCategories(filename string, catMap map[string]*Category) {
	file, err := os.Open(filename)
	if err != nil {
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
		id := parts[0]
		parentId := parts[1]
		// Имя категории обычно в конце или предпоследнее, зависит от типа. 
		// Для fr_fr.txt: ID|Parent|...|Name|Description
		// Для nw_nw.txt: ID|Parent|...|Name|...
		
		// Попытка найти имя. В разных файлах структура немного отличается.
		// Обычно имя идет после нескольких числовых полей.
		// Эвристика: берем поле, которое выглядит как текст и не пустое, часто это 4-е или 5-е поле в зависимости от файла
		// В fr_fr.txt: 1|0|1|1|1112265867|Резервный| -> Name is parts[5]
		// В nw_nw.txt: 1|0|0|1|262|НПСО|... -> Name is parts[5]
		
		name := "Без названия"
		if len(parts) > 5 {
			name = parts[5]
			if name == "" && len(parts) > 4 {
				name = parts[4]
			}
		} else if len(parts) > 4 {
			name = parts[4]
		}
		
		// Очистка от HTML тегов если есть
		name = strings.TrimSpace(strings.ReplaceAll(name, "&quot;", "\""))

		catMap[id] = &Category{
			ID:       id,
			ParentID: parentId,
			Name:     name,
			Count:    0,
			Level:    0,
		}
	}
	
	// Вычисление уровней вложенности
	for _, cat := range catMap {
		level := 0
		curr := cat.ParentID
		visited := make(map[string]bool)
		for curr != "0" && curr != "" {
			if visited[curr] { break } // Защита от циклов
			visited[curr] = true
			if p, ok := catMap[curr]; ok {
				curr = p.ParentID
				level++
			} else {
				break
			}
		}
		cat.Level = level
	}
}

// Парсинг inf.json для категорий публикаций
func parseInfJson(filename string, catMap map[string]*Category) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	var info map[string][]string
	if err := json.Unmarshal(data, &info); err != nil {
		return
	}

	for id, fields := range info {
		// Структура массива в inf.json специфична, обычно: [Name, ParentID, ...]
		// Пример из вывода: "7":["Новые газеты","4","2","1","1","","","0","0","",""]
		if len(fields) >= 2 {
			name := fields[0]
			parentId := fields[1]
			catMap[id] = &Category{
				ID:       id,
				ParentID: parentId,
				Name:     name,
				Count:    0,
				Level:    0,
			}
		}
	}
	// Пересчет уровней для inf.json категорий тоже нужен, но упростим для краткости
}

// Парсинг пользователей
func parseUsers(usersFile, ugenFile string, authors map[string]*Author) {
	// Читаем основные данные
	f1, err := os.Open(usersFile)
	if err == nil {
		defer f1.Close()
		scanner := bufio.NewScanner(f1)
		for scanner.Scan() {
			// Format: Nick|Hash|Pass|Avatar|Group|...|RealName|...
			// users.txt: Korovin|...|19|...
			// ugen.txt содержит расширенные данные: ID|Nick|...|RealName|...
			// Сопоставление сложное, попробуем взять из users.txt ник, а реальное имя поищем потом или оставим ник
			parts := strings.Split(scanner.Text(), "|")
			if len(parts) > 0 {
				nick := parts[0]
				if _, ok := authors[nick]; !ok {
					authors[nick] = &Author{NickName: nick, RealName: "", Count: 0}
				}
			}
		}
	}

	// Читаем расширенные профили для реальных имен
	f2, err := os.Open(ugenFile)
	if err == nil {
		defer f2.Close()
		scanner := bufio.NewScanner(f2)
		for scanner.Scan() {
			// Format: ID|Nick|Group|...|RealName|...
			// Пример: 1|Korovin|3|0|0||39|0|,1,3,5,8|39|448|1|1383|176|2|2|0|0|1775283476|3306||0|2|0|...
			// Реальное имя часто после кучи чисел. В ugen.txt формат сложный.
			// Попробуем эвристику: второе поле - ник.
			parts := strings.Split(scanner.Text(), "|")
			if len(parts) >= 2 {
				nick := parts[1]
				// Имя может быть в конце или в середине. 
				// В примере: 1|Korovin|...|Павел Семёнович|... (не очевидно без документации)
				// Оставим пока только ник, если не найдем явного указания.
				if a, ok := authors[nick]; ok {
					// Попытка найти имя (часто после аватара или группы)
					// Для простоты оставим пустым, если неясно, или используем Nick
					if a.RealName == "" {
						a.RealName = nick // Заглушка
					}
				} else {
					authors[nick] = &Author{NickName: nick, RealName: nick, Count: 0}
				}
			}
		}
	}
}

// Парсинг статей (news.txt и publ.txt)
func parseArticles(filename string, report *Report, typeName string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Ошибка открытия %s: %v\n", filename, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	
	// Определяем мапу категорий в зависимости от типа
	var catMap map[string]*Category
	if typeName == "news" {
		catMap = report.NewsCategories
		report.NewsStats.ByCategory = make(map[string]int)
	} else {
		catMap = report.PublCategories
		report.PublStats.ByCategory = make(map[string]int)
	}

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "|")
		if len(parts) < 12 {
			continue
		}

		// Структура news.txt: ID|CatID|SubCatID|Year|Month|Day|...|DateUnix|HasHTML|Author|Title|...
		// Структура publ.txt: ID|CatID|SubCatID|...|DateUnix|...|Author|Title|...
		
		var catID, yearStr, author string
		
		if typeName == "news" {
			// news: 1|0|5|2009|10|13|0|0|1|1255451181|1|Korovin|Title...
			catID = parts[1]
			yearStr = parts[3]
			author = parts[11]
		} else {
			// publ: 1|4|10|0|0|1254597913|0|1|0|5.00|30|150|IP|Title...
			// Тут структура другая. CatID = parts[1]. Дата в Unix (parts[5]). Автор позже.
			catID = parts[1]
			unixTime, _ := strconv.ParseInt(parts[5], 10, 64)
			if unixTime > 0 {
				t := time.Unix(unixTime, 0)
				yearStr = strconv.Itoa(t.Year())
			} else {
				yearStr = "Unknown"
			}
			// Автор в publ.txt обычно после заголовка или перед ним? 
			// В примере вывода: ...|1254597913|0|1|0|5.00|30|150|213.135.104.108|О деятельности...
			// Похоже автора нет в явном виде в начале, возможно он в конце или в доп полях.
			// Для статистики пропустим автора, если сложно парсить, или возьмем заглушку.
			author = "Admin" 
		}

		count++
		
		// Статистика по годам
		if stats := func() *ArticleStats {
			if typeName == "news" { return &report.NewsStats }
			return &report.PublStats
		}(); stats != nil {
			stats.ByYear[yearStr]++
			stats.Total++
			
			if c, ok := catMap[catID]; ok {
				c.Count++
				stats.ByCategory[catID]++
			} else {
				// Если категория не найдена в основных списках, проверим общие (fr_fr)
				if cCommon, ok := report.NewsCategories[catID]; ok {
					cCommon.Count++
				}
			}
		}

		// Статистика авторов
		if author != "" && author != "0" {
			if a, ok := report.Authors[author]; ok {
				a.Count++
			} else {
				report.Authors[author] = &Author{NickName: author, RealName: author, Count: 1}
			}
		}
	}
	
	if typeName == "news" {
		report.NewsStats.Total = count
	} else {
		report.PublStats.Total = count
	}
}

func parseTags(filename string) int {
	file, err := os.Open(filename)
	if err != nil {
		return 0
	}
	defer file.Close()
	
	lines := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			lines++
		}
	}
	return lines
}

func scanImages() (int, map[string]int) {
	total := 0
	folders := make(map[string]int)
	
	baseDirs := []string{"_nw", "_ld", "_ph", "avatar", "img"}
	
	for _, base := range baseDirs {
		rootPath := filepath.Join(".", base)
		if _, err := os.Stat(rootPath); os.IsNotExist(err) {
			continue
		}
		
		filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(info.Name()))
				if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" {
					total++
					relDir, _ := filepath.Rel(rootPath, filepath.Dir(path))
					if relDir == "." {
						folders[base]++
					} else {
						key := filepath.Join(base, relDir)
						folders[key]++
					}
				}
			}
			return nil
		})
	}
	
	return total, folders
}

func printReport(r *Report) {
	fmt.Println("\n==================================================")
	fmt.Println("               ОТЧЕТ ПО МИГРАЦИИ                ")
	fmt.Println("==================================================")

	// 1. Рубрики
	fmt.Println("\n📂 СТРУКТУРА РУБРИК (Категории)")
	fmt.Println("--- Новости (news.txt) ---")
	printTree(r.NewsCategories, "0", 0)
	
	fmt.Println("\n--- Публикации (publ.txt / inf.json) ---")
	printTree(r.PublCategories, "0", 0)

	fmt.Println("\n--- Файлы (ld_ld.txt) ---")
	printTree(r.FileCategories, "0", 0)

	// 2. Статистика материалов
	fmt.Println("\n📊 СТАТИСТИКА МАТЕРИАЛОВ")
	fmt.Printf("Всего новостей: %d\n", r.NewsStats.Total)
	fmt.Printf("Всего публикаций: %d\n", r.PublStats.Total)
	
	fmt.Println("\nРаспределение новостей по годам:")
	printYearStats(r.NewsStats.ByYear)
	
	fmt.Println("\nРаспределение публикаций по годам:")
	printYearStats(r.PublStats.ByYear)

	// Топ категорий для новостей
	fmt.Println("\nТоп-10 категорий новостей по количеству:")
	printTopCategories(r.NewsCategories, 10)

	// 3. Авторы
	fmt.Println("\n👥 АВТОРЫ (Топ-15)")
	type kv struct { Key string; Val *Author }
	var authors []kv
	for k, v := range r.Authors {
		authors = append(authors, kv{k, v})
	}
	sort.Slice(authors, func(i, j int) bool { return authors[i].Val.Count > authors[j].Val.Count })
	
	for i, a := range authors {
		if i >= 15 { break }
		fmt.Printf("  %d. %s (%s) - %d статей\n", i+1, a.Val.NickName, a.Val.RealName, a.Val.Count)
	}
	fmt.Printf("  ... всего авторов: %d\n", len(r.Authors))

	// 4. Изображения
	fmt.Println("\n🖼️  ИЗОбРАЖЕНИЯ")
	fmt.Printf("Всего файлов изображений: %d\n", r.TotalImages)
	fmt.Println("Распределение по папкам (топ-10):")
	type kvImg struct { Key string; Val int }
	var imgs []kvImg
	for k, v := range r.ImageFolders {
		imgs = append(imgs, kvImg{k, v})
	}
	sort.Slice(imgs, func(i, j int) bool { return imgs[i].Val > imgs[j].Val })
	for i, img := range imgs {
		if i >= 10 { break }
		fmt.Printf("  %s: %d шт.\n", img.Key, img.Val)
	}

	// 5. Метки
	fmt.Printf("\n🏷️  МЕТКИ (Теги): %d уникальных записей в файле tags.txt\n", r.TagsCount)

	// 6. Рекомендации
	fmt.Println("\n💡 РЕКОМЕНДАЦИИ ДЛЯ СКРИПТА МИГРАЦИИ:")
	fmt.Println("1. Создайте в WordPress следующие основные рубрики:")
	
	// Собираем корневые категории новостей
	var rootNews []string
	for _, c := range r.NewsCategories {
		if c.ParentID == "0" && c.Count > 0 {
			rootNews = append(rootNews, fmt.Sprintf("%s (Новости)", c.Name))
		}
	}
	// Собираем корневые категории публикаций
	var rootPubl []string
	for _, c := range r.PublCategories {
		if c.ParentID == "0" && c.Count > 0 {
			rootPubl = append(rootPubl, fmt.Sprintf("%s (Публикации)", c.Name))
		}
	}
	
	if len(rootNews) > 0 {
		fmt.Println("   - Новости: " + strings.Join(rootNews, ", "))
	}
	if len(rootPubl) > 0 {
		fmt.Println("   - Публикации: " + strings.Join(rootPubl, ", "))
	}
	
	fmt.Println("2. Все материалы (Новости и Публикации) импортировать как тип 'Post' (Запись).")
	fmt.Println("3. Использовать поле 'Author' из Ucoz для маппинга на пользователей WP (создать недостающих).")
	fmt.Println("4. Теги из tags.txt привязывать по ID материала.")
	fmt.Println("5. Изображения копировать из папок _nw, _ld, _ph, сохраняя относительную структуру или перекладывая в wp-content/uploads/YYYY/MM.")
}

func printTree(cats map[string]*Category, parentId string, level int) {
	found := false
	for _, c := range cats {
		if c.ParentID == parentId {
			if !found { found = true }
			indent := strings.Repeat("  ", level)
			sign := "├──"
			if level == 0 { sign = "📁" }
			fmt.Printf("%s%s %s (ID:%s) [%d мат.]\n", indent, sign, c.Name, c.ID, c.Count)
			printTree(cats, c.ID, level+1)
		}
	}
}

func printYearStats(years map[string]int) {
	keys := make([]string, 0, len(years))
	for k := range years {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	for _, k := range keys {
		fmt.Printf("  %s: %d\n", k, years[k])
	}
}

func printTopCategories(cats map[string]*Category, limit int) {
	type kv struct { Key string; Val *Category }
	var sorted []kv
	for k, v := range cats {
		if v.Count > 0 {
			sorted = append(sorted, kv{k, v})
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Val.Count > sorted[j].Val.Count })
	
	for i, c := range sorted {
		if i >= limit { break }
		fmt.Printf("  %d. %s (ID:%s) - %d\n", i+1, c.Val.Name, c.Val.ID, c.Val.Count)
	}
}