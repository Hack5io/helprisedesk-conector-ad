// Comando principal del conector -- se instala como Servicio de Windows
// vía github.com/kardianos/service (subcomandos "install"/"uninstall"/
// "start"/"stop"/"run" gratis sobre el mismo binario). install.ps1 solo
// llama a "conector-ad.exe install" después de escribir config.json.
package main

import (
	"flag"
	"log"
	"time"

	"github.com/kardianos/service"

	"github.com/Hack5io/helprisedesk-conector-ad/internal/adldap"
	"github.com/Hack5io/helprisedesk-conector-ad/internal/config"
	"github.com/Hack5io/helprisedesk-conector-ad/internal/poller"
)

type programa struct {
	detener chan struct{}
	cfg     *config.Config
}

func (p *programa) Start(s service.Service) error {
	p.detener = make(chan struct{})
	go p.loop()
	return nil
}

func (p *programa) Stop(s service.Service) error {
	close(p.detener)
	return nil
}

func (p *programa) loop() {
	cliente := &adldap.Cliente{
		Host:             p.cfg.AdHost,
		Dominio:          p.cfg.AdDominio,
		Usuario:          p.cfg.AdUsuario,
		Password:         p.cfg.AdPassword,
		CuentasExcluidas: p.cfg.AdCuentasExcluidas,
	}
	pll := poller.Nuevo(p.cfg.Token, p.cfg.ApiURL, cliente)

	intervalo := time.Duration(p.cfg.PollSegundos) * time.Second
	ticker := time.NewTicker(intervalo)
	defer ticker.Stop()

	for {
		pll.Ciclo()

		select {
		case <-p.detener:
			return
		case <-ticker.C:
		}
	}
}

func main() {
	rutaConfig := flag.String("config", config.RutaPorDefecto(), "Ruta al archivo config.json")
	flag.Parse()

	svcConfig := &service.Config{
		Name:        "HelpriseDeskConectorAD",
		DisplayName: "HelpriseDesk - Conector de Active Directory",
		Description: "Conecta HelpriseDesk con el Active Directory local sin abrir puertos de entrada.",
		Arguments:   []string{"-config", *rutaConfig},
	}

	prg := &programa{}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalf("no se pudo inicializar el servicio: %v", err)
	}

	// "install", "uninstall", "start", "stop" como argumento posicional
	// (no via -config) -- los maneja kardianos/service directamente,
	// mismo patrón que su documentación oficial.
	if len(flag.Args()) > 0 {
		if err := service.Control(s, flag.Args()[0]); err != nil {
			log.Fatalf("no se pudo %s el servicio: %v", flag.Args()[0], err)
		}
		return
	}

	cfg, err := config.Cargar(*rutaConfig)
	if err != nil {
		log.Fatalf("configuración inválida: %v", err)
	}
	prg.cfg = cfg

	if err := s.Run(); err != nil {
		log.Fatalf("el servicio terminó con error: %v", err)
	}
}
