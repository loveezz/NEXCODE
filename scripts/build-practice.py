"""Build the shipped curriculum from the four supplied DOCX documents.

Original task paragraphs are retained verbatim. NEXCODE input/output contracts
are appended separately; free-form work is reviewed by the teacher.
"""
from pathlib import Path
import hashlib
import json
import re
import shutil
import xml.etree.ElementTree as ET
import zipfile

ROOT = Path(__file__).resolve().parents[1]
W = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
SPECS = [
    ("1-2", "ЯП_ЛР1,2.docx", "Словесные, табличные алгоритмы и блок-схемы"),
    ("3-4", "ЯП_ЛР3,4.docx", "Переменные, типы данных, условия и циклы"),
    ("5-6", "ЯП_ЛР5_6.docx", "Функции, процедуры и структуры данных"),
    ("7-8", "ЯП_ЛР7,8.docx", "Работа с файлами и обработка исключений"),
]

def paragraphs(filename):
    with zipfile.ZipFile(ROOT / "practice" / filename) as archive:
        body = ET.fromstring(archive.read("word/document.xml")).find(W+"body")
        return ["".join(n.text or "" for n in p.iter(W+"t")).strip() for p in body if p.tag == W+"p"]

def original_tasks(key, lines):
    result, current, chunks = {}, None, []
    started = False
    for line in lines:
        if line.startswith("3. Ход работы"): started = True; continue
        if not started: continue
        if line.startswith("4. Контрольные вопросы"): break
        pattern = r"^(3\.\d+(?:\.\d+)?)\s" if key in ("1-2","7-8") else r"^(3\.\d+\.\d+)\s"
        match = re.match(pattern, line)
        if key == "3-4" and line.startswith("3.7 Индивидуальное"):
            if current: result[current] = "\n".join(chunks)
            current, chunks = None, []
            continue
        if key == "3-4" and line.startswith(("А)", "Б)")):
            match = None
            if current: result[current] = "\n".join(chunks)
            current, chunks = "3.7"+line[0], [line]
            continue
        if match:
            if current: result[current] = "\n".join(chunks)
            current, chunks = match.group(1), [line]
        elif current and line and line not in ("Индивидуальное задание",):
            chunks.append(line)
    if current: result[current] = "\n".join(chunks)
    return result

DATA, AUTHORS = {}, {}
for key, filename, topic in SPECS:
    lines=paragraphs(filename)
    DATA[key]=original_tasks(key,lines)
    AUTHORS[key]={"file":filename,"sha256":hashlib.sha256((ROOT/"practice"/filename).read_bytes()).hexdigest()}

catalog = {"version":"practice-2026-11-17-v1", "sources":AUTHORS, "labs":[]}
by_lab={}
for position,(key,filename,topic) in enumerate(SPECS,1):
    lab={"key":"practice-"+key,"title":f"Лабораторная работа {key.replace('-', '–')} · {topic}",
         "description":f"{topic}. Задания из {filename}; исходная нумерация сохранена.",
         "position":position,"source":"practice","document_url":f"/practice/lr-{key}.docx","exercises":[]}
    catalog["labs"].append(lab); by_lab[key]=lab

def case(stdin="", stdout="", files=None, expected_files=None, denied_files=None):
    item={"stdin":stdin,"stdout":stdout}
    if files is not None: item["files"]=files
    if expected_files is not None: item["expected_files"]=expected_files
    if denied_files is not None: item["denied_files"]=denied_files
    return item

def add(lab, number, title, contract="", tests=None, template="", kind="python"):
    original=DATA[lab][number]
    if kind=="written":
        contract=contract or "Запишите ответ здесь: словесный алгоритм, таблицу и/или блок-схему. Для схем можно использовать Mermaid (flowchart TD) с предпросмотром. Ответ проверяет преподаватель; автотестов нет."
        tests=[]
    elif kind=="python_review":
        contract += "\n\nДанные и оформление выбирайте по условию. Кнопка «Запустить» позволяет проверить программу. Сдайте код преподавателю: автоматическая оценка за свободный выбор данных не выставляется."
        tests=tests or [case()]
    elif not tests:
        raise ValueError("Autotests required: "+lab+" "+number)
    exercise={"catalog_key":f"practice-{lab}:{number}","title":f"{number}. {title}",
              "statement":original+"\n\nФормат выполнения в NEXCODE\n"+contract,
              "template":template or ("# Напишите ответ\n" if kind=="written" else "# Напишите программу по условию\n"),
              "tests":"\n".join(json.dumps(t,ensure_ascii=False,separators=(",",":")) for t in tests),
              "kind":kind,"timeout_ms":5000,"memory_mb":128,
              "difficulty":"Письменная работа" if kind=="written" else "Индивидуальное" if "Индивидуальное" in original else "Практика",
              "position":len(by_lab[lab]["exercises"])+1}
    by_lab[lab]["exercises"].append(exercise)

titles=["Алгоритм приготовления бутерброда","Переход дороги","Можно ли идти гулять","Стоимость покупки со скидкой",
        "Исправление алгоритма приготовления чая","Команды робота на поле","Блок-схема приготовления бутерброда",
        "Блок-схема перехода дороги","Блок-схема выбора прогулки","Знак числа","Блок-схема факториала","Корзина с яблоками"]
for index,title in enumerate(titles,1):
    template="Мой алгоритм:\n1. \n\nТаблица шагов:\nНомер | Действие | Пояснение\n"
    if index>=7: template='flowchart TD\n    start([Начало]) --> step["Опишите действие"]\n    step --> finish([Конец])\n'
    add("1-2",f"3.{index}",title,template=template,kind="written")

L="3-4"
add(L,"3.1.1","Сведения о маршрутизаторе",kind="python_review")
add(L,"3.1.2","Сведения о канале связи",kind="python_review")
add(L,"3.2.1","Значения переменных и их типы",kind="python_review")
add(L,"3.2.2","Преобразование введённых данных",
    "Три строки: скорость (float), число устройств (int), температура (float). Выведите на отдельных строках значение и type() через пробел. Приглашения ко вводу не выводите.",
    [case("100.5\n12\n65.0\n","100.5 <class 'float'>\n12 <class 'int'>\n65.0 <class 'float'>\n"),
     case("0\n0\n-5.25\n","0.0 <class 'float'>\n0 <class 'int'>\n-5.25 <class 'float'>\n")],
    "speed = float(input())\ndevices = int(input())\ntemperature = float(input())\n# Выведите значения и типы\n")
add(L,"3.3.1","Потери 75 из 5000 пакетов",
    "Ввод отсутствует. Выведите три строки: получено пакетов (целое), процент потерь (2 знака после точки), отношение потерянных к полученным (6 знаков).",
    [case(stdout="4925\n1.50\n0.015228\n")])
add(L,"3.3.2","Объём и скорость передачи",
    "Три строки: пропускная способность в Мбит/с, время в секундах (>0), фактический объём в Мбит. Выведите максимум, разницу и среднюю скорость — по строке, по 2 знака после точки.",
    [case("100\n10\n750\n","1000.00\n250.00\n75.00\n"),case("8\n2.5\n20\n","20.00\n0.00\n8.00\n"),case("2.5\n4\n0\n","10.00\n10.00\n0.00\n")],
    "capacity = float(input())\nduration = float(input())\nactual = float(input())\n")
add(L,"3.4.1","Контроль задержки 100 мс","Одна строка: задержка. Выведите сообщение из условия без точки.",
    [case(str(x)+"\n",("Высокая задержка" if x>100 else "Задержка в норме")+"\n") for x in [45,100,101,0]])
add(L,"3.4.2","Температура и допустимый предел","Две строки: температура и допустимый предел. Перегрев — строго выше предела. Выведите сообщение из условия.",
    [case(f"{a}\n{b}\n",("Перегрев оборудования" if a>b else "Температура в норме")+"\n") for a,b in [(80,70),(70,70),(-5,0)]])
add(L,"3.4.3","Качество канала по задержке","Одна строка: неотрицательная целая задержка. Выведите соответствующее сообщение из условия.",
    [case(f"{x}\n",("Отличное качество" if x<=50 else "Нормальное качество" if x<=100 else "Повышенная задержка" if x<=200 else "Критическая задержка")+"\n") for x in [0,50,51,100,101,200,201]])
add(L,"3.4.4","Классификация потерь пакетов","Две строки: отправлено (>0), потеряно (от 0 до отправлено). Выведите сообщение из условия.",
    [case(f"{s}\n{p}\n",("Потерь нет" if p==0 else "Незначительные потери" if p/s*100<=2 else "Допустимые потери" if p/s*100<=5 else "Высокие потери")+"\n") for s,p in [(100,0),(100,2),(100,3),(100,5),(100,6),(5000,75)]])
add(L,"3.4.5","Комплексная проверка оборудования",
    "Четыре строки: подключено (1/0), температура, отправлено (>0), потеряно. Выведите «Оборудование исправно» либо «Требуется проверка».",
    [case(f"{c}\n{t}\n{s}\n{p}\n",("Оборудование исправно" if c and t<=70 and p/s*100<=5 else "Требуется проверка")+"\n") for c,t,s,p in [(1,70,100,5),(0,40,100,0),(1,71,100,0),(1,40,100,6),(1,40,5000,75)]])
devices=["Router-01","Router-02","Switch-01","Switch-02","AccessPoint-01"]
add(L,"3.5.1","Цикл проверки устройств","Ввод отсутствует. Для каждого имени из условия выведите строку «Начало проверки: ИМЯ».",
    [case(stdout="".join(f"Начало проверки: {x}\n" for x in devices))])
add(L,"3.5.2","Состояние каждой задержки","Ввод отсутствует. Выведите по строке для каждого значения исходного списка, сообщения точно как в условии.",
    [case(stdout="".join(("Проблемная задержка" if x>100 else "Задержка в норме")+"\n" for x in [45,80,120,150,60,95]))])
add(L,"3.5.3","Пакеты с потерями выше 5%","Самостоятельно выберите количество отправленных пакетов и поясните этот выбор в комментарии. Проценты вычисляйте по отправленным и потерянным пакетам.",kind="python_review")
add(L,"3.5.4","Диапазон и чётные числа","Ввод отсутствует. Первая строка — числа 1…50 через пробел; вторая — чётные 2…50 через пробел.",
    [case(stdout=" ".join(map(str,range(1,51)))+"\n"+" ".join(map(str,range(2,51,2)))+"\n")])
add(L,"3.5.5","Пять попыток подключения","Ввод отсутствует. Для каждой попытки — строка «Попытка N». После пятой — «Последняя попытка подключения.».",
    [case(stdout="".join(f"Попытка {n}\n" for n in range(1,6))+"Последняя попытка подключения.\n")])
add(L,"3.5.6","Сумма ошибок","Ввод отсутствует. Выведите одно целое число — сумму исходного списка.",[case(stdout="11\n")])
add(L,"3.5.7","Сумма и средняя задержка","Ввод отсутствует. Две строки: сумма (целое) и среднее (2 знака после точки).",[case(stdout="455\n91.00\n")])
add(L,"3.5.8","Подсчёт перегревов","Ввод отсутствует. Две строки: количество нормальных значений и количество перегревов.",[case(stdout="4\n3\n")])
add(L,"3.6.1","Три попытки с while","Ввод отсутствует. Строки «Попытка 1» … «Попытка 3», затем «Попытки завершены.».",
    [case(stdout="Попытка 1\nПопытка 2\nПопытка 3\nПопытки завершены.\n")])
add(L,"3.6.2","Ввод задержек до -1","Задержки вводятся по одной на строку; -1 завершает ввод. Порог — 100 мс. Для каждого значения выведите одно из двух сообщений условия.",
    [case("45\n100\n101\n-1\n","Задержка в норме\nЗадержка в норме\nВысокая задержка\n"),case("-1\n",""),case("150\n0\n-1\n","Высокая задержка\nЗадержка в норме\n")])
delay_cases=[]
for threshold,values in [(100,[45,80,120,150,60]),(50,[50]),(0,[])]:
    output="Нет данных\n"
    if values:
        normal=sum(x<=threshold for x in values)
        output="Задержки: "+" ".join(map(str,values))+"\n"+"".join(("Норма" if x<=threshold else "Проблема")+"\n" for x in values)+f"Нормальных: {normal}\nПроблемных: {len(values)-normal}\nСумма: {sum(values)}\nСредняя: {sum(values)/len(values):.2f}\nМаксимум: {max(values)}\n"
    delay_cases.append(case(f"{threshold}\n{len(values)}\n"+"".join(f"{x}\n" for x in values),output))
add(L,"3.7А","Итоговый отчёт по задержкам",
    "Ввод: целый порог, n (0…1000), затем n целых задержек по строке. Вывод: «Задержки: » и значения через пробел; далее для каждого «Норма»/«Проблема»; затем строки «Нормальных: N», «Проблемных: N», «Сумма: S», «Средняя: X.XX», «Максимум: M». При n=0 — «Нет данных». Равенство порогу — норма.",delay_cases)
temperature_cases=[]
for threshold,values in [(70,[45,72,68,81,55]),(70,[70]),(0,[])]:
    output="Нет данных\n"
    if values:
        hot=[x for x in values if x>threshold]
        output="Температуры: "+" ".join(map(str,values))+"\n"+"".join(("Перегрев" if x>threshold else "Норма")+"\n" for x in values)+"Перегревы: "+(" ".join(map(str,hot)) or "-")+f"\nПерегревов: {len(hot)}\nНормальных: {len(values)-len(hot)}\nСредняя: {sum(values)/len(values):.2f}\nМаксимум: {max(values)}\n"
    temperature_cases.append(case(f"{threshold}\n"+"".join(f"{x}\n" for x in values)+"-1\n",output))
add(L,"3.7Б","Итоговый отчёт по температурам",
    "Ввод: целый порог, затем целые температуры до -1. Вывод: «Температуры: » и список через пробел; для каждого значения «Норма»/«Перегрев»; «Перегревы: » и значения через пробел (или -); «Перегревов: N», «Нормальных: N», «Средняя: X.XX», «Максимум: M». При пустом списке — «Нет данных». -1 в расчёты не входит.",temperature_cases)

L="5-6"
add(L,"3.1.1","Операции со списком задержек",
    "В исходнике опечатка «на 54 место (после 80)». Ориентируйтесь на уточнение: вставьте 200 сразу после 80. Выведите 4 строки: наличие 120 (True/False), количество, обновлённый список и отсортированный список (обычный вывод Python-списка).",
    [case(stdout="True\n7\n[45, 80, 200, 120, 150, 60, 95]\n[45, 60, 80, 95, 120, 150, 200]\n")],
    "latencies = [45, 110, 80, 120, 150, 60, 95]\n")
add(L,"3.1.2","Операции со списком устройств",
    "Ввод отсутствует. Выведите: исходный список; первое имя; последнее имя; количество после добавления и удаления; наличие Router-02 (True/False); отсортированный итоговый список. По одной строке.",
    [case(stdout=str(devices)+"\nRouter-01\nAccessPoint-01\n5\nTrue\n"+str(sorted([x for x in devices if x!="Switch-01"]+["Firewall-01"]))+"\n")],
    "devices = "+repr(devices)+"\n")
add(L,"3.2.1","Методы словаря устройства",kind="python_review")
add(L,"3.2.2","Изменение и очистка словаря","В условии temperature уже присутствует после 3.2.1: покажите добавление при отсутствии ключа или изменение существующего значения.",kind="python_review")
add(L,"3.2.3","Таблица пяти устройств",kind="python_review")
equipment=[
    {"name":"Router-01","ip":"192.168.1.1","status":"active","latency":45},
    {"name":"Router-02","ip":"192.168.1.2","status":"error","latency":120},
    {"name":"Switch-01","ip":"192.168.1.3","status":"warning","latency":80},
    {"name":"Switch-02","ip":"192.168.1.4","status":"offline","latency":150},
    {"name":"AccessPoint-01","ip":"192.168.1.5","status":"active","latency":60},
]
short_equipment=[{"name":"A","ip":"10.0.0.1","status":"active","latency":100}]
def jsonline(value): return json.dumps(value,ensure_ascii=False)+"\n"
list_template="import json\ndevices = json.loads(input())\n# Ввод списка словарей уже выполнен; решите задание\n"
add(L,"3.2.4","Фильтрация устройств",
    "Для независимой проверки список устройств передаётся одной JSON-строкой (чтение в шаблоне). Выведите 3 строки: «error: » и имена со статусом error; «warning: » и имена warning; «latency: » и имена с задержкой >100. Имена через пробел, при отсутствии — -.",
    [case(jsonline(ds),"\n".join(label+": "+(" ".join(d["name"] for d in ds if predicate(d)) or "-") for label,predicate in [("error",lambda d:d["status"]=="error"),("warning",lambda d:d["status"]=="warning"),("latency",lambda d:d["latency"]>100)])+"\n") for ds in [equipment,short_equipment,[]]],list_template)
add(L,"3.3.1","Поиск устройства с break",
    "Ввод: JSON-список словарей, затем имя. Найдите первое совпадение и выведите «Найдено: ИМЯ IP»; если нет — «Устройство не найдено». Чтение списка готово в шаблоне.",
    [case(jsonline(equipment)+target+"\n",next((f"Найдено: {d['name']} {d['ip']}\n" for d in equipment if d["name"]==target),"Устройство не найдено\n")) for target in ["Router-02","Missing"]],list_template+"target = input()\n")
add(L,"3.3.2","Первая задержка выше 100",
    "Используйте обновлённый список после 3.1.1: 45, 80, 200, 120, 150, 60, 95. Выведите первое значение >100. Ввод отсутствует.",
    [case(stdout="200\n")],"latencies = [45, 80, 200, 120, 150, 60, 95]\n")
add(L,"3.3.3","Пропуск неактивных устройств",
    "Ввод: одна JSON-строка со списком устройств. Выведите только имена со status active, по одному на строке. Если таких нет — ничего.",
    [case(jsonline(ds),"".join(d["name"]+"\n" for d in ds if d["status"]=="active")) for ds in [equipment,[],short_equipment]],list_template)
add(L,"3.4.1","Функция приветствия мониторинга","Создайте функцию без параметров и вызовите дважды. Ввод отсутствует; каждое сообщение с новой строки.",
    [case(stdout=("Система мониторинга сети\nПроверка оборудования запущена\n")*2)])
add(L,"3.4.2","Процедура вывода меню","Ввод отсутствует. Четыре строки: «Список устройств», «Проверка соединения», «Отчёт», «Выход». Создайте и вызовите процедуру.",
    [case(stdout="Список устройств\nПроверка соединения\nОтчёт\nВыход\n")])
add(L,"3.4.3","Функция проверки задержки","Одна строка: задержка. Передайте её в свою функцию и выведите сообщение из условия.",
    [case(f"{x}\n",("Задержка в норме" if x<=100 else "Высокая задержка")+"\n") for x in [0,100,101]])
add(L,"3.4.4","Функция проверки температуры","Одна строка: температура. Порог 70°C, как в работе 3–4; выше порога — «Перегрев», иначе «Температура в норме». Используйте функцию.",
    [case(f"{x}\n",("Перегрев" if x>70 else "Температура в норме")+"\n") for x in [45,70,71]])
add(L,"3.4.5","Функция процента потерь","Две строки: отправлено (>0), потеряно. Верните процент из функции и выведите с 2 знаками после точки.",
    [case(f"{s}\n{p}\n",f"{p/s*100:.2f}\n") for s,p in [(5000,75),(100,0),(100,100),(3,1)]])
add(L,"3.4.6","Функция допустимости задержки","Две строки: задержка и предел. Выведите возвращённое функцией True/False; равенство пределу допустимо.",
    [case(f"{x}\n{p}\n",str(x<=p)+"\n") for x,p in [(100,100),(101,100),(0,1)]])
add(L,"3.4.7","Функция средней задержки","Используйте список из условия. Выведите результат функции с 2 знаками после точки. Ввод отсутствует.",
    [case(stdout="80.00\n")],"latencies = [45, 80, 120, 60, 95]\n")
add(L,"3.4.8","Функция статуса устройства","Ввод: одна JSON-строка со словарём (ключ status присутствует). Верните status из функции и выведите.",
    [case(jsonline(d),d["status"]+"\n") for d in [equipment[0],equipment[1],equipment[2]]],
    "import json\ndevice = json.loads(input())\n")
add(L,"3.4.9","Функция состояния устройства","Ввод: одна JSON-строка со словарём. Выведите результат функции точно как в условии; неизвестный статус — «Норма».",
    [case(jsonline({"status":s}),("Проблема" if s=="error" else "Предупреждение" if s=="warning" else "Норма")+"\n") for s in ["error","warning","active","offline"]],
    "import json\ndevice = json.loads(input())\n")
add(L,"3.4.10","Функция подсчёта ошибок","Ввод: JSON-список словарей. Выведите целое число, возвращённое функцией.",[case(jsonline(ds),str(sum(d["status"]=="error" for d in ds))+"\n") for ds in [equipment,[],[equipment[1],equipment[1]]]],list_template)
add(L,"3.4.11","Функция списка проблемных имён","Ввод: JSON-список словарей. Функция возвращает список имён с latency >100. Выведите их по одному на строке; если список пуст — «Нет устройств».",
    [case(jsonline(ds),"".join(d["name"]+"\n" for d in ds if d["latency"]>100) or "Нет устройств\n") for ds in [equipment,[],short_equipment]],list_template)
add(L,"3.5.1","Функции мониторинга устройств","Выберите собственные устройства и значения полей; покажите хотя бы одно исправное и одно проблемное устройство. Поясните правило проблемного состояния.",kind="python_review")
add(L,"3.5.2","Функции анализа измерений","Сохраните пороги из условия: задержка >100 мс или потери >5%. Данные списка выберите самостоятельно; проверьте также граничные значения.",kind="python_review")
add(L,"3.6.1","Рекурсивный факториал","Одна строка: n (0…20). Выведите n!, вычисленный рекурсивной функцией.",
    [case(f"{n}\n",str(__import__("math").factorial(n))+"\n") for n in [0,1,5,10,20]])
add(L,"3.6.2","Рекурсивные квадраты до нуля","Одна строка: n (0…100). Выведите квадраты от n до 0 в одной строке через пробел; используйте рекурсию.",
    [case(f"{n}\n"," ".join(str(x*x) for x in range(n,-1,-1))+"\n") for n in [5,0,1,10]])

L="7-8"
lines=paragraphs("ЯП_ЛР7,8.docx")
log_start=lines.index("2026-10-07 10:15:03 Router-01 INFO Link up",lines.index("3.7.1 Дан журнал событий logs.log. Создайте report.txt и перенесите в него только строки с ошибками."))
log_lines=[]
for line in lines[log_start:]:
    if re.match(r"^2026-\d\d-\d\d \d\d:",line): log_lines.append(line)
    elif line and log_lines: break
LOG="\n".join(log_lines)+"\n"
long_start=lines.index("2026-10-07 08:00:01 Router-01 INFO Link up")
long_lines=[]
for line in lines[long_start:]:
    if re.match(r"^2026-\d\d-\d\d \d\d:",line): long_lines.append(line)
    elif line and long_lines: break
LONG_LOG="\n".join(long_lines)+"\n"
assert len(log_lines)==12 and len(long_lines)==50
file_note="Файлы учебные и хранятся в памяти отдельного запуска. Используйте open() с относительным именем и encoding='utf-8'; файлы сервера недоступны. Исходные файлы показаны рядом с редактором."
add(L,"3.1","Организация рабочей папки",
    "В браузере основной main.py уже открыт в редакторе. Запишите дерево своей рабочей папки, названия программ и будущих текстовых файлов и кратко поясните их назначение. Это письменная организационная часть, её проверяет преподаватель.",kind="written",template="Структура рабочей папки:\n\nНазначение файлов:\n")
add(L,"3.2","Запись и чтение сведений об устройстве",file_note+"\nПоля и значения выберите самостоятельно; запишите network_info.txt и прочитайте его обратно. Ориентируйтесь на пример из документа.",kind="python_review")
add(L,"3.3","Построчное чтение пяти устройств",file_note+"\nВ листинге приведены три примера имени, но условие требует пять устройств. Самостоятельно дополните список до пяти и создайте devices.txt.",kind="python_review")
measurement_cases=[]
for values in [[45,80,120,150],[0,100,101],[]]:
    measurement_cases.append(case(files={"measurements.txt":"".join(f"{x}\n" for x in values)},stdout="".join(f"{x}\n" for x in values if x>100)))
add(L,"3.4","Чтение задержек из файла",file_note+"\nФайл measurements.txt подготовлен для каждого теста. Прочитайте его и выведите значения строго выше 100, каждое с новой строки.",measurement_cases)
small_log="2026-10-07 10:15:03 Router-01 INFO Link up\n2026-10-07 10:16:12 Router-01 WARNING High latency: 125 ms\n2026-10-07 10:17:45 Router-02 ERROR Link down\n"
log_cases=[]
for value in [small_log,LOG,""]:
    rows=value.splitlines()
    output="ERROR:\n"+"".join(x+"\n" for x in rows if "ERROR" in x)+"WARNING:\n"+"".join(x+"\n" for x in rows if "WARNING" in x)+f"Количество WARNING: {sum('WARNING' in x for x in rows)}\n"
    log_cases.append(case(stdout=output,files={"network.log":value}))
add(L,"3.5","Ошибки и предупреждения журнала",file_note+"\nnetwork.log подготовлен. Выведите «ERROR:», затем строки с ERROR; «WARNING:», затем строки с WARNING; в конце «Количество WARNING: N». Сохраните порядок строк.",log_cases)
add(L,"3.6","Режимы записи w и a",file_note+"\nСоздайте report36.txt в режиме w, затем дополните в режиме a. Данные отчёта выберите самостоятельно. Преподаватель проверит сохранение исходных записей.",kind="python_review")
report_cases=[]
stats_cases=[]
for value in [LOG,"2026-10-07 10:00:00 Router-01 INFO Link up\n",""]:
    rows=value.splitlines(); errors=[x for x in rows if "ERROR" in x]
    report="ОШИБКИ СЕТЕВОГО ОБОРУДОВАНИЯ\nДата формирования отчёта: 2026-10-07\n\n"+"".join(x+"\n" for x in errors)
    report_cases.append(case(files={"logs.log":value},expected_files={"report.txt":report}))
    statistics=f"СТАТИСТИЧЕСКИЙ ОТЧЁТ ПО ЖУРНАЛУ СЕТИ\nДата формирования отчёта: 2026-10-07\n\nОбщее количество строк журнала: {len(rows)}\nКоличество сообщений ERROR: {sum('ERROR' in x for x in rows)}\nКоличество сообщений WARNING: {sum('WARNING' in x for x in rows)}\n"
    stats_cases.append(case(files={"logs.log":value},expected_files={"report.txt":statistics}))
add(L,"3.7.1","Файл отчёта об ошибках",file_note+"\nlogs.log подготовлен. Создайте report.txt: заголовок «ОШИБКИ СЕТЕВОГО ОБОРУДОВАНИЯ», строка «Дата формирования отчёта: 2026-10-07», пустая строка, затем только ERROR-строки журнала. Дата фиксирована как в исходном примере. В stdout ничего выводить не нужно.",report_cases)
add(L,"3.7.2","Статистический отчёт в файле",file_note+"\nlogs.log подготовлен. Создайте report.txt точно как в примере исходного документа: заголовок «СТАТИСТИЧЕСКИЙ ОТЧЁТ ПО ЖУРНАЛУ СЕТИ», дата 2026-10-07, пустая строка, затем «Общее количество строк журнала: N», «Количество сообщений ERROR: N», «Количество сообщений WARNING: N». В stdout ничего выводить не нужно.",stats_cases)
add(L,"3.8.1","Обработка ошибки целого числа",
    "Одна строка. При успешном int() выведите число, при ValueError — «Ошибка: введите целое число». Используйте try/except.",
    [case("42\n","42\n"),case("-7\n","-7\n"),case("abc\n","Ошибка: введите целое число\n"),case("2.5\n","Ошибка: введите целое число\n")])
add(L,"3.8.2","Обработка ошибки температуры",
    "Одна строка: корректное конечное число или текст. При успешном float() выведите число с 2 знаками после точки, при ValueError — «Ошибка: введите число».",
    [case("36.6\n","36.60\n"),case("-5\n","-5.00\n"),case("тепло\n","Ошибка: введите число\n")])
add(L,"3.9.1","Обработка отсутствующего файла",file_note+"\nИмя файла вводится одной строкой. При успехе выведите содержимое, при FileNotFoundError — «Файл не найден».",
    [case("devices.txt\n","Router-01\n",files={"devices.txt":"Router-01\n"}),case("missing.txt\n","Файл не найден\n",files={})])
add(L,"3.9.2","Проверка наличия файла",file_note+"\nИмя файла вводится. Можно использовать os.path.exists() или pathlib.Path(...).exists(): они работают только с учебными файлами. Покажите проверку до чтения и понятное сообщение.",
    [case("devices.txt\n",files={"devices.txt":"Router-01\n"})],kind="python_review")
add(L,"3.10","Индивидуальный анализ журнала",file_note+"\nВ учебной среде подготовлен networkLogs.log из 50 строк исходного документа. Прочитайте его, выведите ошибки, посчитайте ERROR/WARNING и сформируйте report.txt. Добавьте обработку FileNotFoundError, PermissionError и ошибок записи. Отчёт и структуру обработки проверяет преподаватель.",
    [case(files={"networkLogs.log":LONG_LOG})],kind="python_review")

assert [len(l["exercises"]) for l in catalog["labs"]]==[12,23,24,13]
for key,source in DATA.items():
    actual={e["catalog_key"].split(":",1)[1] for e in by_lab[key]["exercises"]}
    assert actual==set(source), (key,actual.symmetric_difference(source))
catalog["version"]="practice-"+hashlib.sha256(json.dumps({"sources":AUTHORS,"labs":catalog["labs"]},ensure_ascii=False,sort_keys=True).encode()).hexdigest()[:16]
target=ROOT/"backend/internal/store/catalog"
target.mkdir(parents=True,exist_ok=True)
(target/"practice.json").write_text(json.dumps(catalog,ensure_ascii=False,indent=2)+"\n",encoding="utf-8",newline="\n")
importable={"labs":[{k:v for k,v in lab.items() if k!="key"} for lab in catalog["labs"]]}
(ROOT/"practice/catalog.json").write_text(json.dumps(importable,ensure_ascii=False,indent=2)+"\n",encoding="utf-8",newline="\n")
(ROOT/"practice/manifest.json").write_text(json.dumps({"version":catalog["version"],"sources":AUTHORS},ensure_ascii=False,indent=2)+"\n",encoding="utf-8",newline="\n")
preview={"labs":[],"exercises":[]}
eid=1
for lid,lab in enumerate(catalog["labs"],1):
    preview["labs"].append({k:v for k,v in dict(lab,id=lid).items() if k not in ("exercises","key")})
    for exercise in lab["exercises"]:
        preview["exercises"].append(dict(exercise,id=eid,lab_id=lid,status="not_started",test_count=len(exercise["tests"].splitlines()) if exercise["kind"]=="python" else 0))
        eid+=1
(ROOT/"frontend/src/data/preview-catalog.json").write_text(json.dumps(preview,ensure_ascii=False,indent=2)+"\n",encoding="utf-8",newline="\n")
documents=ROOT/"frontend/public/practice"
documents.mkdir(parents=True,exist_ok=True)
for key,filename,_ in SPECS: shutil.copyfile(ROOT/"practice"/filename,documents/f"lr-{key}.docx")
print("Built 4 labs / 72 exercises:",{kind:sum(e["kind"]==kind for l in catalog["labs"] for e in l["exercises"]) for kind in ["python","python_review","written"]})
