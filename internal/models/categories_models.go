package models

import "time"

type categories struct{
	ID 		  uint 		`json:"id" gorm:"primaryKey"`
	Name 	  string	`json:"name" gorm:"size:100;not null;uniqueIndex"`
	CreatedAt time.Time	`json:"created_at"`
}