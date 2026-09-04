package handlers

import "github.com/gofiber/fiber/v2"

// Item is a single item record.
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Health reports service health.
func Health(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

// Status reports service status.
func Status(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

// ListItems returns all items.
func ListItems(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON([]Item{})
}

// CreateItem creates an item.
func CreateItem(c *fiber.Ctx) error {
	var it Item
	if err := c.BodyParser(&it); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	return c.Status(fiber.StatusCreated).JSON(it)
}

// GetItem returns one item.
func GetItem(c *fiber.Ctx) error {
	id := c.Params("itemID")
	if id == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return c.Status(fiber.StatusOK).JSON(Item{ID: id})
}
