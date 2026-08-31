package todos

import "time"

type Todo struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CreateInput struct {
	Title string `json:"title" binding:"required,min=1,max=200"`
	Done  *bool  `json:"done"`
}

type UpdateInput struct {
	Title *string `json:"title" binding:"omitempty,min=1,max=200"`
	Done  *bool   `json:"done"`
}
