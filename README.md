
---

# 🧠 gitfame

> CLI-утилита для подсчёта вклада авторов в Git-репозитории: строки, коммиты, файлы.
> Вдохновлено [git-fame](https://github.com/casperdcl/git-fame), но реализовано на Go.

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)

---

## 🔍 Что делает gitfame?

`gitfame` позволяет узнать, кто сколько строк, файлов и коммитов добавил в проект — в удобной табличке или JSON/CSV формате.
Это как `git blame`, только с человеческим лицом 😎

Пример:

```bash
gitfame --repository=. --extensions='.go,.md' --order-by=lines
```

```
Name           Lines  Commits  Files
Joe Tsai       12154     92      49
Roger Peppe       59      1       2
...
```

---

## ⚙️ Использование

```bash
gitfame [flags]
```

### Основные флаги:

| Флаг              | Описание                                              |
| ----------------- | ----------------------------------------------------- |
| `--repository`    | Путь к git-репозиторию (по умолчанию: `.`)            |
| `--revision`      | Коммит или ветка для анализа (по умолчанию: `HEAD`)   |
| `--extensions`    | Фильтрация по расширениям файлов, например `.go,.md`  |
| `--languages`     | Языки по типу `go,markdown`                           |
| `--order-by`      | Ключ сортировки: `lines` \| `commits` \| `files`      |
| `--use-committer` | Считать по коммиттеру, а не автору                    |
| `--format`        | Формат вывода: `tabular`, `csv`, `json`, `json-lines` |
| `--exclude`       | Исключить файлы по glob-паттернам                     |
| `--restrict-to`   | Анализировать только соответствующие паттерну файлы   |
| `--progress`      | Показывать прогресс в stderr                          |

---

## 📦 Примеры

### CSV-вывод:

```bash
gitfame --format=csv
```

```csv
Name,Lines,Commits,Files
Joe Tsai,64,3,2
Ross Light,2,1,1
```

### Только `.go` файлы:

```bash
gitfame --extensions='.go'
```

