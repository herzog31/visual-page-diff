package main

import (
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"github.com/scorredoira/email"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	pages         []string
	pagesHash     []string
	threshold     float64
	width         uint64
	height        uint64
	scale         float64
	fuzz          uint64
	interval      uint64
	smtp_user     string
	smtp_password string
	smtp_host     string
	smtp_from     string
	smtp_to       string
)

func main() {
	pages = make([]string, 0)

	parseEnv()
	prepareHashes()

	scanPages()
	for range time.Tick(time.Duration(interval) * time.Second) {
		go scanPages()
	}

}

func scanPages() {
	for k, page := range pages {
		scanPage(page, pagesHash[k])
	}
	return
}

func scanPage(page string, hash string) {

	currentScreen := fmt.Sprintf("%s.png", hash)
	oldScreen := fmt.Sprintf("%s_old.png", hash)
	diffScreen := fmt.Sprintf("%s_diff.png", hash)

	log.Printf("Begin scan of %s (%s).\n", page, hash)

	// 1. Get screenshot
	out, err := exec.Command("docker", "run", "--rm", "-v", "/output:/raster-output", "herzog31/rasterize", page, currentScreen, fmt.Sprintf("%dpx*%dpx", width, height), fmt.Sprintf("%f", scale)).CombinedOutput()
	if err != nil {
		log.Printf("Error while executing the rasterize container for page %s: %v %s", page, err, out)
		return
	}

	// Compare only if older version exists
	if _, err := os.Stat("/output/" + oldScreen); err == nil {
		// 2. Compare current screenshot with old one
		out, err = exec.Command("docker", "run", "--rm", "-v", "/output:/images", "herzog31/imagemagick", "compare", "-verbose", "-metric", "AE", "-fuzz", fmt.Sprintf("%d%%", fuzz), oldScreen, currentScreen, diffScreen).CombinedOutput()
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					if status.ExitStatus() != 1 { // ImageMagick Compare return 1 if images are different and 0 if they are the same
						log.Printf("Error while executing the imagemagick container for page %s: %v %s", page, err, out)
						return
					}
				}
			} else {
				log.Printf("Error while executing the imagemagick container for page %s: %v %s", page, err, out)
				return
			}
		}

		// 3. Parse verbose output
		var diff uint64
		lines := strings.Split(string(out), "\n")
		for _, l := range lines {
			if strings.Contains(l, "all:") {
				number := strings.Split(l, ":")[1]
				number = strings.TrimSpace(number)
				diff, _ = strconv.ParseUint(number, 10, 64)
			}
		}

		// 4. Calculate change in percentage
		change := float64(diff) / float64(height*width)

		// 5. If change > threshold, notify
		if change > 0 && change < threshold {
			log.Printf("Change of %s detected (%f%%), but does not exceed threshold of %f%% lines.\n", page, change*100.0, threshold*100.0)
		} else if change > threshold {
			log.Printf("Change of %s detected! (%f%%)\n", page, change*100.0)
			err := sendNotification(page, "/output/"+diffScreen)
			if err != nil {
				log.Printf("Error while sending notification for page %s: %v\n", page, err)
				return
			}
		} else {
			log.Printf("No change of %s detected.\n", page)
		}
	} else {
		log.Printf("First scan of %s was successful, no previous screenshot for comparison available.\n", page)
	}

	// 6. Remove old screenshot & diff
	if _, err := os.Stat("/output/" + oldScreen); err == nil {
		err = os.Remove("/output/" + oldScreen)
		if err != nil {
			log.Printf("Error while removing old screenshot: %v", err)
			return
		}
	}
	if _, err := os.Stat("/output/" + diffScreen); err == nil {
		err = os.Remove("/output/" + diffScreen)
		if err != nil {
			log.Printf("Error while removing diff screenshot: %v", err)
			return
		}
	}

	// 7. Rename current screenshot
	err = os.Rename("/output/"+currentScreen, "/output/"+oldScreen)
	if err != nil {
		log.Printf("Error while renaming current screenshot: %v", err)
		return
	}

	return
}

func sendNotification(page string, image string) error {

	from := mail.Address{"", smtp_from}
	to := mail.Address{"", smtp_to}

	m := email.NewMessage(fmt.Sprintf("Change detected: %s", page), fmt.Sprintf("Change on page %s detected:\n\n", page))
	m.From = smtp_from
	m.To = []string{smtp_to}
	err := m.Attach(image)
	if err != nil {
		return err
	}

	// Connect to the SMTP Server
	host, _, _ := net.SplitHostPort(smtp_host)
	auth := smtp.PlainAuth("", smtp_user, smtp_password, host)

	// TLS config
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	conn, err := tls.Dial("tcp", smtp_host, tlsconfig)
	if err != nil {
		return err
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}

	// Auth
	if err = c.Auth(auth); err != nil {
		return err
	}

	// To && From
	if err = c.Mail(from.Address); err != nil {
		return err
	}

	if err = c.Rcpt(to.Address); err != nil {
		return err
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(m.Bytes())
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	c.Quit()

	return nil

}

func parseEnv() {
	// Pages
	env_pages := os.Getenv("PAGES")
	if env_pages == "" {
		log.Fatal("Environment variable PAGES is empty.")
	}
	pages = append(pages, strings.Split(env_pages, ",")...)

	// Interval
	env_interval := os.Getenv("INTERVAL")
	if env_interval == "" {
		log.Fatal("Environment variable INTERVAL is empty.")
	}
	interval_parsed, err := strconv.ParseUint(env_interval, 10, 64)
	if err != nil {
		log.Fatal("Environment variable INTERVAL is no valid integer.")
	}
	interval = interval_parsed

	// Threshold
	env_threshold := os.Getenv("THRESHOLD")
	if env_threshold != "" {
		threshold_parsed, err := strconv.ParseFloat(env_threshold, 64)
		if err != nil {
			log.Fatal("Environment variable THRESHOLD is no valid integer.")
		}
		threshold = threshold_parsed
	}

	// WIDTH
	env_width := os.Getenv("WIDTH")
	if env_width != "" {
		width_parsed, err := strconv.ParseUint(env_width, 10, 64)
		if err != nil {
			log.Fatal("Environment variable WIDTH is no valid integer.")
		}
		width = width_parsed
	}

	// HEIGHT
	env_height := os.Getenv("HEIGHT")
	if env_height != "" {
		height_parsed, err := strconv.ParseUint(env_height, 10, 64)
		if err != nil {
			log.Fatal("Environment variable HEIGHT is no valid integer.")
		}
		height = height_parsed
	}

	// FUZZ
	env_fuzz := os.Getenv("FUZZ")
	if env_fuzz != "" {
		fuzz_parsed, err := strconv.ParseUint(env_fuzz, 10, 64)
		if err != nil {
			log.Fatal("Environment variable FUZZ is no valid integer.")
		}
		fuzz = fuzz_parsed
	}

	// SCALE
	env_scale := os.Getenv("SCALE")
	if env_scale != "" {
		scale_parsed, err := strconv.ParseFloat(env_scale, 64)
		if err != nil {
			log.Fatal("Environment variable SCALE is no valid integer.")
		}
		scale = scale_parsed
	}

	// Mail
	env_smtp_user := os.Getenv("SMTP_USER")
	if env_smtp_user == "" {
		log.Fatal("Environment variable SMTP_USER is empty.")
	}
	smtp_user = env_smtp_user

	env_smtp_password := os.Getenv("SMTP_PASSWORD")
	if env_smtp_password == "" {
		log.Fatal("Environment variable SMTP_PASSWORD is empty.")
	}
	smtp_password = env_smtp_password

	env_smtp_host := os.Getenv("SMTP_HOST")
	if env_smtp_host == "" {
		log.Fatal("Environment variable SMTP_HOST is empty.")
	}
	smtp_host = env_smtp_host

	env_smtp_from := os.Getenv("SMTP_FROM")
	if env_smtp_from == "" {
		log.Fatal("Environment variable SMTP_FROM is empty.")
	}
	smtp_from = env_smtp_from

	env_smtp_to := os.Getenv("SMTP_TO")
	if env_smtp_to == "" {
		log.Fatal("Environment variable SMTP_TO is empty.")
	}
	smtp_to = env_smtp_to
}

func prepareHashes() {
	pagesHash = make([]string, len(pages))
	for k, p := range pages {
		hash := md5.Sum([]byte(p))
		pagesHash[k] = fmt.Sprintf("%x", hash)
	}
}
