
package tickettype

import (
    "errors"
    "log"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}

func (h *Handler) Create(c *gin.Context) {
    // Get event ID from URL
    eventID, err := strconv.ParseInt(
        c.Param("id"), 10, 64,
    )

    if err != nil || eventID <= 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "invalid event id",
        })
        return
    }

    // Read JSON request
    var input CreateInput

    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "invalid request data",
        })
        return
    }

    // Call service
    ticket, err := h.service.Create(
        c.Request.Context(),
        eventID,
        input,
    )

    if err != nil {
        switch {
        case errors.Is(err, ErrInvalidTicketType):
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "invalid ticket type data",
            })

        case errors.Is(err, ErrEventNotFound):
            c.JSON(http.StatusNotFound, gin.H{
                "error": "event not found",
            })

        case errors.Is(err, ErrTicketTypeExists):
            c.JSON(http.StatusConflict, gin.H{
                "error": "ticket type already exists",
            })

        default:
            log.Printf("create ticket type: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "internal server error",
            })
        }
        return
    }

    c.JSON(http.StatusCreated, gin.H{
        "message": "ticket type created successfully",
        "data":    ticket,
    })
}
