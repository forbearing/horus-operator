package mysql

import "gorm.io/gorm"

type Resource struct {
	gorm.Model

	// resource type, such as: deployment, pod, statefulset, etc.
	Type    string `gorm:"varchar(20);not null" json:"type"`
	Data    string `gorm:"longtext;not null" json:"data"`
	PVCName string `gorm:"varchar(255)" json:"pvc_name"`
	PVName  string `gorm:"varchar(255)" json:"pv_name"`
}

func (r *Resource) Create() {}
func (r *Resource) Update() {}
func (r *Resource) Read()   {}
func (r *Resource) Delete() {}
