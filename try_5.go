package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
	func imprimirArgumentos(args []string) {
		fmt.Println("Argumentos:")
		for i, arg := range args {
			fmt.Printf("%d: %s\n", i+1, arg)
		}
	}
*/
func main() {
	rand.Seed(time.Now().UnixNano())

	args := os.Args[1:]
	if len(args) != 10 {
		fmt.Println("Uso: go run main.go -m [valor_m] -p [valor_p] -orden [archivo_orden_creacion_procesos] -salida [archivo_salida] -n [cantidad_nucleos]")
		return
	}

	m, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Println("El valor de m debe ser un número válido.")
		return
	}

	pStr := args[3]
	p, err := strconv.ParseFloat(pStr, 64)
	if err != nil {
		fmt.Println("El valor de p debe ser un número válido.")
		return
	}

	ordenEjecucion := args[5]
	archivoSalida := args[7]

	if err := verificarArchivosProcesos(); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Archivos de procesos verificados correctamente (OK)")

	if _, err := os.Stat(ordenEjecucion); os.IsNotExist(err) {
		fmt.Println("El archivo de orden de ejecución no existe.")
		return
	}

	fmt.Println("Archivo de orden de ejecución verificado correctamente (OK)")

	if err := verificarCrearArchivoSalida(archivoSalida, m, p); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Archivo de salida verificado y/o creado correctamente (OK)")

	nucleos, err := strconv.Atoi(args[9])
	if err != nil {
		fmt.Println("El valor de n debe ser un número válido.")
		return
	}

	var wg sync.WaitGroup

	// Crear un canal para coordinar la finalización de las goroutines
	finalizarProcesoChan := make(chan struct{})

	for i := 0; i < nucleos; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			procesoEnEjecucionChan := make(chan string, nucleos)
			simularConNucleo(id, nucleos, m, p, ordenEjecucion, archivoSalida, procesoEnEjecucionChan, finalizarProcesoChan)
		}(i)
	}

	wg.Wait()

	// Cerrar el canal finalizarProcesoChan para indicar que todas las goroutines han terminado
	close(finalizarProcesoChan)
}

func simularConNucleo(nucleoID, totalNucleos, m int, p float64, ordenEjecucion, archivoSalida string, procesoEnEjecucionChan chan string, finalizarProcesoChan chan struct{}) {
	fmt.Printf("Iniciando simularConNucleo para núcleo %d\n", nucleoID)

	procesos := make(map[string][]string)
	estadoProceso := make(map[string]string)
	contadorInstrucciones := make(map[string]int)
	bloqueoES := make(map[string]int)

	for proceso := range procesoEnEjecucionChan {
		fmt.Printf("Núcleo %d: Recibido proceso %s\n", nucleoID, proceso)

		if _, ok := procesos[proceso]; !ok {
			for {
				instrucciones, err := cargarInstruccionesProceso(proceso)
				if err != nil {
					log.Printf("Error al cargar instrucciones para proceso %s: %v\n", proceso, err)
					time.Sleep(time.Millisecond * 100)
					continue
				}
				procesos[proceso] = instrucciones
				estadoProceso[proceso] = "Listo"
				contadorInstrucciones[proceso] = 0
				break
			}
		}

		estadoProceso[proceso] = "Ejecutando"
		fmt.Printf("Núcleo %d: Proceso %s establecido a Ejecutando\n", nucleoID, proceso)

		for {
			instrucciones := procesos[proceso]
			contador := contadorInstrucciones[proceso]

			if contador >= len(instrucciones) {
				estadoProceso[proceso] = "Terminado"
				escribirTraza(archivoSalida, contador, proceso, "Terminado", nucleoID)
				fmt.Printf("Núcleo %d: Proceso %s ha terminado\n", nucleoID, proceso)
				break
			}

			instruccion := instrucciones[contador]
			if strings.HasPrefix(instruccion, "ES") {
				duracionES, err := strconv.Atoi(strings.TrimPrefix(instruccion, "ES"))
				if err != nil {
					log.Printf("Error al interpretar instrucción E/S para proceso %s: %v\n", proceso, err)
					continue
				}

				bloqueoES[proceso] = duracionES
				estadoProceso[proceso] = "Bloqueado"
				fmt.Printf("Núcleo %d: Proceso %s bloqueado por E/S durante %d ciclos.\n", nucleoID, proceso, duracionES)
				escribirTraza(archivoSalida, contador, proceso, "Bloqueado por E/S", nucleoID)
			} else {
				fmt.Printf("Núcleo %d: Proceso %s ejecutando instrucción: %s\n", nucleoID, proceso, instruccion)
				contadorInstrucciones[proceso]++
				escribirTraza(archivoSalida, contador, proceso, instruccion, nucleoID)
			}

			if contador%(m+1) == 0 {
				estadoProceso[proceso] = "Listo"
				fmt.Printf("Núcleo %d: Proceso %s ha alcanzado el límite de ejecución y se moverá de nuevo a listo.\n", nucleoID, proceso)
				break
			}
		}

	}

	fmt.Printf("Núcleo %d: Todos los procesos han terminado\n", nucleoID)
}

func simularPrincipal(m int, p float64, ordenEjecucion, archivoSalida string, procesoEnEjecucionChan, finalizarProcesoChan chan string, totalNucleos int) {
	// Inicializar otras variables necesarias para la simulación principal
	orden, err := cargarOrdenEjecucion(ordenEjecucion)
	if err != nil {
		fmt.Println(err)
		return
	}

	procesos := make(map[string][]string)    // Mapa para almacenar instrucciones de procesos
	estadoProceso := make(map[string]string) // Mapa para almacenar el estado de los procesos
	procesosListos := make([]string, 0)      // Cola de procesos listos
	cicloCPU := 1

	// Crear un canal para indicar que todas las goroutines han terminado
	done := make(chan struct{})

	// Crear y lanzar goroutines para cada núcleo
	for i := 0; i < totalNucleos; i++ {
		go simularConNucleo(i, totalNucleos, m, p, ordenEjecucion, archivoSalida, procesoEnEjecucionChan, done)
	}

	for {
		// Verificar si hay procesos listos y núcleos disponibles
		if len(procesosListos) > 0 {
			for nucleoID := 0; nucleoID < totalNucleos; nucleoID++ {
				if len(procesosListos) > 0 {
					proceso := procesosListos[0]
					procesoEnEjecucionChan <- proceso
					procesosListos = procesosListos[1:]
				}
			}
		}

		// Cargar procesos nuevos de acuerdo al ciclo de CPU
		for _, o := range orden {
			if o.TiempoCreacion == cicloCPU {
				proceso := o.NombreProceso
				fmt.Printf("Nuevo proceso cargado: %s\n", proceso)
				instrucciones, err := cargarInstruccionesProceso(proceso)
				if err != nil {
					fmt.Println("Error al cargar instrucciones del proceso:", err)
					return
				}
				procesos[proceso] = instrucciones
				estadoProceso[proceso] = "Listo"
				procesosListos = append(procesosListos, proceso)
			}
		}

		// Verificar si se ha completado la simulación
		if todosTerminados(estadoProceso) {
			break
		}

		// Incrementar el ciclo de CPU
		cicloCPU++
		time.Sleep(time.Millisecond)
	}

	// Esperar a que todas las goroutines de simularConNucleo finalicen antes de terminar la simulación
	<-done
	close(procesoEnEjecucionChan)
	close(finalizarProcesoChan)
}

type OrdenEjecucion struct {
	TiempoCreacion int
	NombreProceso  string
}

func cargarOrdenEjecucion(archivoOrden string) ([]OrdenEjecucion, error) {
	contenido, err := ioutil.ReadFile(archivoOrden)
	if err != nil {
		return nil, fmt.Errorf("Error al leer el archivo de orden de ejecución: %v", err)
	}

	lineas := strings.Split(string(contenido), "\n")
	var orden []OrdenEjecucion

	for _, linea := range lineas {
		if strings.TrimSpace(linea) == "" {
			continue
		}

		partes := strings.Split(linea, "|")
		if len(partes) != 2 {
			return nil, fmt.Errorf("Formato incorrecto en la línea: '%s'", linea)
		}

		tiempoCreacion, err := strconv.Atoi(strings.TrimSpace(partes[0]))
		if err != nil {
			return nil, fmt.Errorf("Error al convertir el tiempo de creación: %v", err)
		}

		nombreProceso := strings.TrimSpace(partes[1])

		orden = append(orden, OrdenEjecucion{
			TiempoCreacion: tiempoCreacion,
			NombreProceso:  nombreProceso,
		})
	}
	/* Imprime la lista ordenada con los valores filtrados
	fmt.Println("Lista ordenada con valores filtrados:")
	for _, o := range orden {
		fmt.Printf("Tiempo de Creación: %d, Nombre de Proceso: %s\n", o.TiempoCreacion, o.NombreProceso)
	}
	*/

	return orden, nil
}

func cargarInstruccionesProceso(nombreProceso string) ([]string, error) {
	archivoProceso := filepath.Join("procesos", nombreProceso)
	contenido, err := ioutil.ReadFile(archivoProceso)
	if err != nil {
		return nil, fmt.Errorf("Error al cargar las instrucciones del proceso %s: %v", nombreProceso, err)
	}

	lineas := strings.Split(string(contenido), "\n")
	instrucciones := []string{}

	for _, linea := range lineas {
		if strings.TrimSpace(linea) == "" || strings.HasPrefix(linea, "#") {
			continue
		}

		instrucciones = append(instrucciones, linea)
	}

	// Imprimir las instrucciones filtradas
	fmt.Println("Instrucciones del proceso", nombreProceso+":")
	for _, instruccion := range instrucciones {
		fmt.Println(instruccion)
	}

	return instrucciones, nil
}

func obtenerProcesoListo(estadoProceso map[string]string) string {
	for proceso, estado := range estadoProceso {
		if estado == "Listo" {
			return proceso
		}
	}
	return ""
}

func escribirTraza(archivoSalida string, tiempoCPU int, nombreProceso, instruccion string, nucleoID int) {
	traza, err := os.OpenFile(archivoSalida, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error al abrir el archivo de salida: %v", err)
	}
	defer traza.Close()

	_, err = traza.WriteString(fmt.Sprintf("%d\t%s\t%s\t%d\n", tiempoCPU, nombreProceso, instruccion, nucleoID))
	if err != nil {
		log.Fatalf("Error al escribir en el archivo de salida: %v", err)
	}
}

func todosTerminados(estadoProceso map[string]string) bool {
	for _, estado := range estadoProceso {
		if estado != "Terminado" {
			return false
		}
	}
	return true
}

/*
	func simular(m int, p float64, ordenEjecucion, archivoSalida string) {
		// Inicializar el generador de números aleatorios
		rand.Seed(time.Now().UnixNano())

		fmt.Println("Cargando orden de ejecución de los procesos...")
		orden, err := cargarOrdenEjecucion(ordenEjecucion)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println("Abriendo archivo de salida para escribir la traza...")
		traza, err := os.Create(archivoSalida)
		if err != nil {
			fmt.Println("Error al abrir el archivo de salida:", err)
			return
		}
		defer traza.Close()

		fmt.Println("Iniciando simulación...")
		procesos := make(map[string][]string)
		estadoProceso := make(map[string]string)
		contadorInstrucciones := make(map[string]int)
		indiceOrden := 0
		procesoEnEjecucion := ""
		cicloCPU := 1
		nucleoID := 0

		for {
			//fmt.Println(orden[indiceOrden].NombreProceso)
			//fmt.Println(orden[indiceOrden].TiempoCreacion)
			if indiceOrden < len(orden) && orden[indiceOrden].TiempoCreacion == cicloCPU {
				proceso := orden[indiceOrden].NombreProceso
				fmt.Printf("Cargando instrucciones para el proceso: %s\n", proceso)
				instrucciones, err := cargarInstruccionesProceso(proceso)
				if err != nil {
					fmt.Println("Error al cargar instrucciones del proceso:", err)
					return
				}
				procesos[proceso] = instrucciones
				estadoProceso[proceso] = "Listo"
				contadorInstrucciones[proceso] = 0
				fmt.Printf("Proceso %s creado y listo para ejecución.\n", proceso)
				indiceOrden++
			}

			if procesoEnEjecucion == "" {
				procesoEnEjecucion = obtenerProcesoListo(estadoProceso)
				if procesoEnEjecucion != "" {
					estadoProceso[procesoEnEjecucion] = "Ejecutando"
					fmt.Printf("Proceso %s está ahora ejecutando.\n", procesoEnEjecucion)
				}
			}

			if procesoEnEjecucion != "" {
				instrucciones := procesos[procesoEnEjecucion]
				contador := contadorInstrucciones[procesoEnEjecucion]

				if strings.HasPrefix(instrucciones[contador], "ES") {
					cantidadES, _ := strconv.Atoi(strings.TrimPrefix(instrucciones[contador], "ES"))
					fmt.Printf("Proceso %s ejecutando instrucción E/S, bloqueado por %d ciclos.\n", procesoEnEjecucion, cantidadES)
					estadoProceso[procesoEnEjecucion] = "Bloqueado"
					contadorInstrucciones[procesoEnEjecucion]++
					procesoEnEjecucion = ""
					continue
				}

				fmt.Printf("Proceso %s ejecutando instrucción: %s\n", procesoEnEjecucion, instrucciones[contador])
				contadorInstrucciones[procesoEnEjecucion]++

				if contador == len(instrucciones)-1 {
					fmt.Printf("Proceso %s ha finalizado.\n", procesoEnEjecucion)
					estadoProceso[procesoEnEjecucion] = "Terminado"
					escribirTraza(archivoSalida, cicloCPU, procesoEnEjecucion, "Finalizar", nucleoID)
					procesoEnEjecucion = ""
				} else if contador%(m+1) == 0 {
					fmt.Printf("Proceso %s ha alcanzado el límite de ejecución y se moverá de nuevo a listo.\n", procesoEnEjecucion)
					estadoProceso[procesoEnEjecucion] = "Listo"
					procesoEnEjecucion = ""
				}
			}

			for proceso, estado := range estadoProceso {
				if estado == "Ejecutando" {
					instrucciones := procesos[proceso]
					contador := contadorInstrucciones[proceso]
					if contador < len(instrucciones) {
						escribirTraza(archivoSalida, cicloCPU, procesoEnEjecucion, "Finalizar", nucleoID)
					}
				}
			}

			if todosTerminados(estadoProceso) {
				fmt.Println("Todos los procesos han terminado. Simulación finalizada.")
				break
			}

			if procesoEnEjecucion != "" && rand.Float64() < p {
				fmt.Printf("Proceso %s terminado prematuramente debido a una probabilidad de terminación.\n", procesoEnEjecucion)
				estadoProceso[procesoEnEjecucion] = "Terminado"
				escribirTraza(archivoSalida, cicloCPU, procesoEnEjecucion, "Finalizar", nucleoID)
				procesoEnEjecucion = ""
			}

			cicloCPU++
			time.Sleep(time.Millisecond)
		}
	}
*/
func verificarArchivosProcesos() error {
	// Obtener la lista de archivos en la carpeta "procesos"
	archivos, err := ioutil.ReadDir("procesos")
	if err != nil {
		return fmt.Errorf("Error al leer la carpeta de procesos: %v", err)
	}

	for _, archivo := range archivos {
		// Verificar que el archivo sea un archivo de texto (.txt)
		if !strings.HasSuffix(archivo.Name(), ".txt") {
			continue
		}

		// Verificar que el archivo exista
		if _, err := os.Stat(filepath.Join("procesos", archivo.Name())); os.IsNotExist(err) {
			return fmt.Errorf("El archivo %s no existe.", archivo.Name())
		}
	}

	return nil
}

func verificarArchivoOrden(archivoOrden string) error {
	contenido, err := ioutil.ReadFile(archivoOrden)
	if err != nil {
		return fmt.Errorf("Error al leer el archivo de orden de ejecución: %v", err)
	}

	lineas := strings.Split(string(contenido), "\n")
	if len(lineas) > 0 && strings.TrimSpace(lineas[0]) != fmt.Sprintf("#%s", filepath.Base(archivoOrden)) {
		return fmt.Errorf("El archivo de orden de ejecución no está configurado correctamente.")
	}

	return nil
}

func verificarCrearArchivoSalida(archivoSalida string, m int, p float64) error {
	contenido, err := ioutil.ReadFile(archivoSalida)
	if err != nil {
		// El archivo no existe, crearlo con los valores de m y p
		if err := ioutil.WriteFile(archivoSalida, []byte(fmt.Sprintf("m=%d\np=%.2f\n", m, p)), 0644); err != nil {
			return fmt.Errorf("Error al crear el archivo de salida: %v", err)
		}
		return nil
	}

	// El archivo ya existe, verificar si está vacío
	if len(strings.TrimSpace(string(contenido))) == 0 {
		// El archivo está vacío, escribir los valores de m y p
		if err := ioutil.WriteFile(archivoSalida, []byte(fmt.Sprintf("m=%d\np=%.2f\n", m, p)), 0644); err != nil {
			return fmt.Errorf("Error al escribir en el archivo de salida: %v", err)
		}
		return nil
	}

	return nil
}
