package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TransactionStatus mendefinisikan tipe data eksplisit untuk konstanta status alur transaksi escrow[cite: 2].
type TransactionStatus string

const (
	StatusPendingPayment TransactionStatus = "pending_payment"
	StatusPaidEscrow     TransactionStatus = "paid_escrow"
	StatusShipped        TransactionStatus = "shipped"
	StatusCompleted      TransactionStatus = "completed"
	StatusCancelled      TransactionStatus = "cancelled"
)

// Transaction merepresentasikan entitas utama transaksi escrow pada tabel transactions di PostgreSQL[cite: 2].
// Berkas ini mencakup informasi pembeli, total pembayaran, ongkos kirim hasil kalkulasi API RajaOngkir,
// status pembayaran escrow, nomor resi pengiriman, serta ID transaksi Payment Gateway Midtrans[cite: 2].
type Transaction struct {
	ID              uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	BuyerID         uuid.UUID         `gorm:"type:uuid;not null;index" json:"buyer_id"`
	Buyer           *User             `gorm:"foreignKey:BuyerID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"buyer,omitempty"`
	TotalAmount     float64           `gorm:"type:numeric(15,2);not null;default:0.00" json:"total_amount"`
	ShippingCost    float64           `gorm:"type:numeric(15,2);not null;default:0.00" json:"shipping_cost"`
	DiscountAmount  float64           `gorm:"type:numeric(15,2);not null;default:0.00" json:"discount_amount"`
	VoucherCode     string            `gorm:"type:varchar(50)" json:"voucher_code,omitempty"`
	Status          TransactionStatus `gorm:"type:varchar(50);default:'pending_payment';not null" json:"status"`
	ReceiptNumber   string            `gorm:"type:varchar(100)" json:"receipt_number"`
	MidtransOrderID string            `gorm:"type:varchar(255);uniqueIndex;not null" json:"midtrans_order_id"`
	SnapToken       string            `gorm:"type:varchar(255)" json:"snap_token,omitempty"`
	SnapRedirectURL string            `gorm:"type:text" json:"snap_redirect_url,omitempty"`
	Items           []TransactionItem `gorm:"foreignKey:TransactionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"items,omitempty"`
	CreatedAt       time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara eksplisit untuk entitas Transaction[cite: 2, 4].
func (Transaction) TableName() string {
	return "transactions"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4
// sebelum operasi insert basis data dijalankan[cite: 4].
func (t *Transaction) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// TransactionItem merepresentasikan rincian entitas barang daur ulang atau produk yang dibeli
// pada tabel transaction_items di PostgreSQL[cite: 2].
type TransactionItem struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TransactionID uuid.UUID `gorm:"type:uuid;not null;index" json:"transaction_id"`
	ProductID     uuid.UUID `gorm:"type:uuid;not null;index" json:"product_id"`
	Product       *Product  `gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"product,omitempty"`
	Quantity      int       `gorm:"type:integer;not null;default:1" json:"quantity"`
	Price         float64   `gorm:"type:numeric(15,2);not null;default:0.00" json:"price"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName menetapkan penamaan skema tabel PostgreSQL secara eksplisit untuk entitas TransactionItem[cite: 2, 4].
func (TransactionItem) TableName() string {
	return "transaction_items"
}

// BeforeCreate merupakan life-cycle hook GORM untuk mengeksekusi otomatisasi penciptaan UUID v4
// sebelum operasi insert basis data dijalankan[cite: 4].
func (ti *TransactionItem) BeforeCreate(tx *gorm.DB) (err error) {
	if ti.ID == uuid.Nil {
		ti.ID = uuid.New()
	}
	return nil
}
