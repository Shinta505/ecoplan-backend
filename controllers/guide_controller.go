package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// WasteGuide mendefinisikan struktur data panduan penanganan sampah
type WasteGuide struct {
	Category       string   `json:"category"`
	NameID         string   `json:"name_id"`
	WasteGroup     string   `json:"waste_group"` // Organik, Non-Organik, Residu / B3
	Description    string   `json:"description"`
	HandlingSteps  []string `json:"handling_steps"`
	DosAndDonts    DosDonts `json:"dos_and_donts"`
	RecyclingIdeas []string `json:"recycling_ideas"`
}

// DosDonts mendefinisikan hal yang disarankan (Dos) dan dilarang (Donts)
type DosDonts struct {
	Dos   []string `json:"dos"`
	Donts []string `json:"donts"`
}

// CustomGuideRequest mendefinisikan payload masukan untuk prompt kustom ke Gemini API
type CustomGuideRequest struct {
	Category string `json:"category" binding:"required"`
	Prompt   string `json:"prompt" binding:"required"`
}

// Struct pendukung untuk payload REST Request ke Gemini API
type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

// Struct pendukung untuk merespons payload dari Gemini API
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// Validasi 10 Kategori Sampah Resmi EcoPlan
var validCategories = map[string]bool{
	"battery":    true,
	"biological": true,
	"cardboard":  true,
	"clothes":    true,
	"glass":      true,
	"metal":      true,
	"paper":      true,
	"plastic":    true,
	"shoes":      true,
	"trash":      true,
}

// Database internal data panduan untuk 10 kategori sampah
var wasteGuidesData = map[string]WasteGuide{
	"battery": {
		Category:    "battery",
		NameID:      "Baterai (Limbah B3)",
		WasteGroup:  "Residu / B3",
		Description: "Baterai bekas mengandung logam berat berbahaya seperti merkuri, timbal, kadmium, dan nikel yang dapat mencemari tanah dan air tanah jika dibuang sembarangan.",
		HandlingSteps: []string{
			"Pisahkan baterai dari sampah rumah tangga biasa.",
			"Lapisi kedua kutub baterai (+ dan -) dengan selotip bening/kertas untuk mencegah arus pendek.",
			"Simpan dalam wadah kering ber-merek non-konduktif (seperti kotak plastik atau dus tebal).",
			"Setorkan ke drop-point limbah B3 lokal atau Bank Sampah terdekat.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Simpan di tempat kering dan jauh dari jangkauan anak-anak.",
				"Serahkan ke fasilitas pengolahan limbah B3 resmi.",
			},
			Donts: []string{
				"Dilarang membuang baterai ke tempat sampah umum.",
				"Dilarang membakar baterai karena berisiko meledak dan melepaskan gas beracun.",
				"Dilarang membongkar atau merusak fisik baterai.",
			},
		},
		RecyclingIdeas: []string{
			"Didaur ulang oleh industri pemurnian logam khusus untuk mengekstrak nikel, cobalt, dan lithium.",
		},
	},
	"biological": {
		Category:    "biological",
		NameID:      "Sampah Biologis / Organik",
		WasteGroup:  "Organik",
		Description: "Sampah sisa makhluk hidup seperti sisa makanan, dedaunan, buah/sayur busuk, dan bahan dapur organik yang mudah terurai secara alami.",
		HandlingSteps: []string{
			"Pisahkan dari sampah anorganik (seperti plastik dan kaca).",
			"Tiriskan kandungan air sisa makanan untuk mengendalikan kelembapan dan bau.",
			"Cacah sisa organik menjadi potongan kecil untuk mempercepat proses komposting.",
			"Olah menjadi pupuk kompos menggunakan metode Takakura, biopori, atau komposter anaerob.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Manfaatkan sebagai bahan utama pembuatan kompos padat dan cair.",
				"Gunakan sisa organik sebagai bahan pakan budidaya maggot BSF (Black Soldier Fly).",
			},
			Donts: []string{
				"Jangan mencampur sampah biologis dengan limbah anorganik atau cairan kimia beracun.",
				"Jangan membiarkan sampah biologis membusuk terbuka di dalam ruangan.",
			},
		},
		RecyclingIdeas: []string{
			"Pembuatan Pupuk Organik Cair (POC) dan Kompos Padat.",
			"Bahan pakan pupuk biologis maggot BSF.",
			"Pembuatan Eco-Enzyme dari kulit buah segar dan sisa sayuran.",
		},
	},
	"cardboard": {
		Category:    "cardboard",
		NameID:      "Kardus / Karton",
		WasteGroup:  "Non-Organik",
		Description: "Material kertas tebal bergelombang yang umum digunakan sebagai kemasan barang. Memiliki nilai daur ulang tinggi jika dijaga dalam kondisi kering dan bersih.",
		HandlingSteps: []string{
			"Lepaskan pita perekat (lakban/selotip) dan klip besi dari kardus.",
			"Pipihkan kardus untuk menghemat ruang penyimpanan.",
			"Pastikan kardus tetap bersih dan tidak tercemar minyak atau cairan.",
			"Ikat rapi kardus dan setorkan ke bank sampah atau pengepul daur ulang.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Pipihkan dan lipat kardus sebelum disimpan.",
				"Jaga agar kardus selalu dalam keadaan kering.",
			},
			Donts: []string{
				"Jangan membakar kardus karena menambah emisi emisi karbon udara.",
				"Jangan mencampur kardus basah atau berminyak dengan kardus bersih.",
			},
		},
		RecyclingIdeas: []string{
			"Kerajinan tangan (DIY organizer, kotak penyimpanan, maket).",
			"Pengolahan ulang di pabrik menjadi bubur kertas (pulp) dan kemasan daur ulang.",
		},
	},
	"clothes": {
		Category:    "clothes",
		NameID:      "Pakaian / Tekstil",
		WasteGroup:  "Non-Organik",
		Description: "Limbah produk sandang seperti pakaian bekas, kain perca, dan bahan tekstil yang sudah tidak terpakai.",
		HandlingSteps: []string{
			"Cuci dan bersihkan pakaian hingga higienis.",
			"Sortir pakaian berdasarkan tingkat kelayakan (layak pakai vs rusak).",
			"Pakaian layak pakai dapat didonasikan atau dijual kembali melalui fitur marketplace EcoPlan.",
			"Pakaian rusak dipotong menjadi kain lap (majun) atau bahan kerajinan tekstil.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Gunakan kembali pakaian (upcycling) atau donasikan secara berkelanjutan.",
				"Manfaatkan pakaian rusak sebagai bahan dasar kerajinan kain perca.",
			},
			Donts: []string{
				"Jangan langsung membuang pakaian ke Tempat Pemrosesan Akhir (TPA).",
				"Dilarang membakar kain bergaya sintetis (poliester/nilon).",
			},
		},
		RecyclingIdeas: []string{
			"Pembuatan tas belanja kain (tote bag) dari celana jeans bekas.",
			"Kerajinan keset kaki, sarung bantal, dan kain pembersih (majun).",
		},
	},
	"glass": {
		Category:    "glass",
		NameID:      "Kaca",
		WasteGroup:  "Non-Organik",
		Description: "Material keras transparan seperti botol kaca, toples, dan pecahan kaca. Kaca dapat didaur ulang berulang kali tanpa mengurangi kualitasnya.",
		HandlingSteps: []string{
			"Bilas sisa makanan atau minuman dari dalam kemasan kaca.",
			"Pisahkan tutup botol yang terbuat dari bahan logam atau plastik.",
			"Bungkus pecahan kaca menggunakan kertas koran tebal/kardus dan beri penanda 'AWAS PECAHAN KACA'.",
			"Kumpulkan botol kaca utuh untuk disetorkan ke bank sampah.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Gunakan kembali botol atau toples kaca utuh untuk wadah penyimpanan serbaguna.",
				"Bungkus rapi pecahan kaca demi keselamatan petugas kebersihan.",
			},
			Donts: []string{
				"Jangan membuang pecahan kaca terbuka tanpa pelindung ke wadah sampah biasa.",
			},
		},
		RecyclingIdeas: []string{
			"Vas bunga, tempat lilin, atau pot tanaman hidroponik.",
			"Dilebur kembali oleh industri manufaktur menjadi botol/wadah kaca baru.",
		},
	},
	"metal": {
		Category:    "metal",
		NameID:      "Logam / Kaleng",
		WasteGroup:  "Non-Organik",
		Description: "Sampah berbasis logam seperti kaleng aluminium minuman, kaleng makanan berbahan seng/besi, aluminium foil, dan paku bekas.",
		HandlingSteps: []string{
			"Bersihkan kaleng dari sisa isi makanan atau minuman.",
			"Tekan atau pipihkan kaleng aluminium untuk menghemat ruang wadah.",
			"Pisahkan jenis logam berdasarkan jenisnya (aluminium, besi, tembaga).",
			"Setorkan ke bank sampah atau fasilitas daur ulang logam.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Pastikan wadah logam bersih dari bahan kimia atau minyak.",
				"Pipihkan kaleng aluminium minuman.",
			},
			Donts: []string{
				"Jangan mencampur wadah bekas bahan kimia berbahaya dengan kaleng makanan.",
			},
		},
		RecyclingIdeas: []string{
			"Wadah alat tulis, tempat pensil, atau pot tanaman hias.",
			"Dilebur kembali menjadi bahan baku industri manufaktur logam.",
		},
	},
	"paper": {
		Category:    "paper",
		NameID:      "Kertas",
		WasteGroup:  "Non-Organik",
		Description: "Sampah berbahan kertas lembaran seperti HVS, kertas koran, majalah, dokumen bekas, dan brosur.",
		HandlingSteps: []string{
			"Pisahkan kertas bersih dari kertas basah, berminyak, atau terkena sisa makanan.",
			"Lepaskan klip kertas, stapler, dan jilid plastik dari dokumen.",
			"Tumpuk rapi kertas tanpa meremasnya menjadi bola.",
			"Simpan di tempat yang aman dan kering sebelum disetorkan ke bank sampah.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Gunakan kedua sisi kertas (double-sided print/write) untuk penghematan.",
				"Daur ulang kertas menjadi kertas daur ulang artistik.",
			},
			Donts: []string{
				"Jangan mencampur kertas dengan sampah basah atau berminyak.",
			},
		},
		RecyclingIdeas: []string{
			"Pembuatan kertas daur ulang buatan tangan (hand-made recycled paper).",
			"Kerajinan bubur kertas (papier-mâché).",
		},
	},
	"plastic": {
		Category:    "plastic",
		NameID:      "Plastik",
		WasteGroup:  "Non-Organik",
		Description: "Material sintetis seperti botol PET, kantong plastik (kresek), wadah makanan, dan kemasan saset.",
		HandlingSteps: []string{
			"Bilas dan kosongkan sisa isi dari wadah/botol plastik.",
			"Lepaskan label plastik dan pisahkan tutup botolnya.",
			"Pipihkan botol plastik untuk menghemat ruang volume sampah.",
			"Kumpulkan dan pilah berdasarkan kode plastik (PET, HDPE, PP) jika memungkinkan.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Gunakan kembali wadah plastik yang masih tebal dan bersih untuk keperluan non-konsumsi.",
				"Setorkan botol plastik ke bank sampah atau Reverse Vending Machine (RVM).",
			},
			Donts: []string{
				"Dilarang membakar sampah plastik karena menghasilkan racun dioksin yang berbahaya.",
				"Jangan membuang sampah plastik ke saluran air atau sungai.",
			},
		},
		RecyclingIdeas: []string{
			"Pembuatan Ecobrick untuk bahan bangunan non-struktural.",
			"Pot tanaman gantung dari botol plastik PET bekas.",
			"Kerajinan tas dan dompet dari pilinan kemasan saset.",
		},
	},
	"shoes": {
		Category:    "shoes",
		NameID:      "Sepatu",
		WasteGroup:  "Non-Organik",
		Description: "Limbah alas kaki berbahan kombinasi karet, kulit sintetis, busa, dan tekstil.",
		HandlingSteps: []string{
			"Bersihkan sepatu dari lumpur dan kotoran yang menempel.",
			"Pisahkan sepatu layak pakai dengan sepatu yang sudah rusak berat.",
			"Sepatu yang masih layak dapat disumbangkan atau dijual melalui toko thrift.",
			"Untuk sepatu rusak, pisahkan komponen sol karet untuk proses daur ulang khusus.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Perbaiki (repair) kerusakan ringan pada sepatu sebelum memutuskan membuang.",
				"Sumbangkan sepatu yang sudah tidak terpakai tetapi dalam kondisi baik.",
			},
			Donts: []string{
				"Jangan membuang sepatu ke tempat pembuangan sampah liar.",
			},
		},
		RecyclingIdeas: []string{
			"Dilebur menjadi materi karet pelapis lintasan atletik atau lantai lapangan olahraga.",
			"Penggunaan sol/sepatu boots bekas sebagai pot tanaman estetik (planter).",
		},
	},
	"trash": {
		Category:    "trash",
		NameID:      "Residu / Sampah Umum",
		WasteGroup:  "Residu",
		Description: "Sampah yang tidak dapat didaur ulang kembali secara teknis maupun ekonomis, seperti pembalut, popok sekali pakai, tisu bekas, puntung rokok, atau kemasan multilayer.",
		HandlingSteps: []string{
			"Bungkus sampah residu ke dalam kantong plastik tertutup rapat.",
			"Pastikan tidak mencemari wadah daur ulang organik maupun anorganik.",
			"Tempatkan pada bak sampah residu untuk diangkut oleh petugas kebersihan ke TPA.",
		},
		DosAndDonts: DosDonts{
			Dos: []string{
				"Kurangi penggunaan produk sekali pakai untuk meminimalkan volume sampah residu.",
				"Bungkus dengan rapat dan higienis untuk limbah popok/pembalut.",
			},
			Donts: []string{
				"Jangan membakar sampah residu di lingkungan tempat tinggal.",
				"Jangan membuang sampah residu ke sungai atau selokan.",
			},
		},
		RecyclingIdeas: []string{
			"Pengolahan berbasis bahan bakar alternatif Refuse Derived Fuel (RDF) di fasilitas TPA modern.",
		},
	},
}

// GuideController mengelola endpoint panduan pengolahan sampah & integrasi AI Gemini
type GuideController struct{}

// NewGuideController mengembalikan instance controller baru
func NewGuideController() *GuideController {
	return &GuideController{}
}

// GetAllGuides (GET /api/v1/guides)
// Mengembalikan seluruh daftar panduan dari 10 kategori sampah resmi
func (gc *GuideController) GetAllGuides(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Berhasil mengambil seluruh panduan penanganan sampah",
		"total":   len(wasteGuidesData),
		"data":    wasteGuidesData,
	})
}

// GetGuideByCategory (GET /api/v1/guides/:category)
// Mengembalikan panduan penanganan sampah berdasarkan kategori spesifik
func (gc *GuideController) GetGuideByCategory(c *gin.Context) {
	categoryParam := strings.ToLower(strings.TrimSpace(c.Param("category")))

	if !validCategories[categoryParam] {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Kategori '%s' tidak valid. Kategori yang didukung adalah 10 jenis: battery, biological, cardboard, clothes, glass, metal, paper, plastic, shoes, trash.", categoryParam),
			"valid_categories": []string{
				"battery", "biological", "cardboard", "clothes", "glass", "metal", "paper", "plastic", "shoes", "trash",
			},
		})
		return
	}

	guide, exists := wasteGuidesData[categoryParam]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Panduan untuk kategori sampah tersebut tidak ditemukan.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Berhasil mengambil panduan untuk kategori '%s'", categoryParam),
		"data":    guide,
	})
}

// AskGemini (POST /api/v1/guides/ask)
// Meneruskan prompt kustom pengguna ke Gemini API (model gemini-3.5-flash-lite)
func (gc *GuideController) AskGemini(c *gin.Context) {
	var req CustomGuideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Payload request tidak valid. Pastikan 'category' dan 'prompt' diisi.",
			"error":   err.Error(),
		})
		return
	}

	categoryLower := strings.ToLower(strings.TrimSpace(req.Category))
	if !validCategories[categoryLower] {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Kategori '%s' tidak valid. Gunakan salah satu dari 10 kategori resmi: battery, biological, cardboard, clothes, glass, metal, paper, plastic, shoes, trash.", categoryLower),
		})
		return
	}

	apiKey := os.Getenv("API_KEY_GEMINI")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Konfigurasi server error: API_KEY_GEMINI belum diset pada environment.",
		})
		return
	}

	// Konstruksi System Prompt & User Context untuk Gemini
	systemPrompt := fmt.Sprintf(
		"Kamu adalah asisten ahli pengelolaan limbah dan daur ulang untuk platform EcoPlan.\n"+
			"Kategori Sampah: %s\n"+
			"Pertanyaan/Permintaan Pengguna: %s\n\n"+
			"Petunjuk Jawaban:\n"+
			"1. Berikan penjelasan yang edukatif, praktis, ramah lingkungan, dan aman.\n"+
			"2. Fokus pada metode daur ulang, kreasi produk upcycling, atau langkah penanganan yang benar.\n"+
			"3. Gunakan bahasa Indonesia yang terstruktur, mudah dipahami, serta gaya bahasanya santai seperti ngobrol dengan teman.",
		categoryLower, req.Prompt,
	)

	// Persiapan Payload JSON untuk Gemini REST API
	geminiReqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: systemPrompt},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(geminiReqBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memproses payload request.",
			"error":   err.Error(),
		})
		return
	}

	// Pemanggilan API Gemini dengan model gemini-3.5-flash-lite
	geminiEndpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash-lite:generateContent?key=%s",
		apiKey,
	)

	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest("POST", geminiEndpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal membuat HTTP Request ke Gemini API.",
			"error":   err.Error(),
		})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"status":  "error",
			"message": "Gagal terhubung ke layanan Gemini API.",
			"error":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal membaca respons dari layanan Gemini API.",
			"error":   err.Error(),
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{
			"status":  "error",
			"message": "Gemini API mengembalikan respons error.",
			"details": string(respBodyBytes),
		})
		return
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBodyBytes, &geminiResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal melakukan unmarshal respons JSON dari Gemini API.",
			"error":   err.Error(),
		})
		return
	}

	// Ekstraksi jawaban dari Candidates
	var resultText string
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		resultText = geminiResp.Candidates[0].Content.Parts[0].Text
	} else {
		resultText = "Tidak ada jawaban yang dihasilkan oleh model Gemini."
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Berhasil mendapatkan rekomendasi dari Gemini AI",
		"model":   "gemini-3.5-flash-lite",
		"data": gin.H{
			"category": categoryLower,
			"prompt":   req.Prompt,
			"response": resultText,
		},
	})
}
