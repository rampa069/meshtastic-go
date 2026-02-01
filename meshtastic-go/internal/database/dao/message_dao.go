package dao

import (
	"database/sql"
	"fmt"
	"time"
)

// MessageEntity represents a message/packet record in the database
type MessageEntity struct {
	UUID         int64
	MyNodeNum    uint32
	PortNum      int32
	ContactKey   string // Format: "nodeNum" or "channel:index"
	ReceivedTime int64
	Read         bool
	Data         string // JSON encoded message data
	PacketId     uint32
	RoutingError int32
	SNR          float64
	RSSI         int32
	HopsAway     int32
	Filtered     bool
}

// ReactionEntity represents a reaction to a message
type ReactionEntity struct {
	MyNodeNum    uint32
	ReplyId      uint32
	UserId       string
	Emoji        string
	Timestamp    int64
	SNR          float64
	RSSI         int32
	HopsAway     int32
	PacketId     uint32
	Status       int32
	RoutingError int32
}

// MessageDAO provides data access operations for messages
type MessageDAO struct {
	db *sql.DB
}

// NewMessageDAO creates a new MessageDAO
func NewMessageDAO(db *sql.DB) *MessageDAO {
	return &MessageDAO{db: db}
}

// Insert creates a new message record
func (d *MessageDAO) Insert(msg *MessageEntity) (int64, error) {
	query := `
		INSERT INTO packet (
			myNodeNum, port_num, contact_key, received_time, read, data,
			packet_id, routing_error, snr, rssi, hopsAway, filtered
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := d.db.Exec(query,
		msg.MyNodeNum, msg.PortNum, msg.ContactKey, msg.ReceivedTime,
		boolToInt(msg.Read), msg.Data, msg.PacketId, msg.RoutingError,
		msg.SNR, msg.RSSI, msg.HopsAway, boolToInt(msg.Filtered),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Update updates an existing message record
func (d *MessageDAO) Update(msg *MessageEntity) error {
	query := `
		UPDATE packet SET
			myNodeNum = ?, port_num = ?, contact_key = ?, received_time = ?,
			read = ?, data = ?, packet_id = ?, routing_error = ?,
			snr = ?, rssi = ?, hopsAway = ?, filtered = ?
		WHERE uuid = ?
	`
	result, err := d.db.Exec(query,
		msg.MyNodeNum, msg.PortNum, msg.ContactKey, msg.ReceivedTime,
		boolToInt(msg.Read), msg.Data, msg.PacketId, msg.RoutingError,
		msg.SNR, msg.RSSI, msg.HopsAway, boolToInt(msg.Filtered), msg.UUID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("message %d not found", msg.UUID)
	}
	return nil
}

// Delete removes a message by UUID
func (d *MessageDAO) Delete(uuid int64) error {
	result, err := d.db.Exec("DELETE FROM packet WHERE uuid = ?", uuid)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("message %d not found", uuid)
	}
	return nil
}

// GetByUUID retrieves a message by UUID
func (d *MessageDAO) GetByUUID(uuid int64) (*MessageEntity, error) {
	query := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet WHERE uuid = ?
	`
	row := d.db.QueryRow(query, uuid)
	return scanMessage(row)
}

// GetByPacketId retrieves a message by packet ID
func (d *MessageDAO) GetByPacketId(packetId uint32) (*MessageEntity, error) {
	query := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet WHERE packet_id = ? LIMIT 1
	`
	row := d.db.QueryRow(query, packetId)
	return scanMessage(row)
}

// GetByContact retrieves all messages for a contact
func (d *MessageDAO) GetByContact(contactKey string, limit int) ([]*MessageEntity, error) {
	query := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet
		WHERE contact_key = ?
		ORDER BY received_time DESC
		LIMIT ?
	`
	return d.queryMessages(query, contactKey, limit)
}

// GetByContactAfter retrieves messages for a contact after a timestamp
func (d *MessageDAO) GetByContactAfter(contactKey string, after int64, limit int) ([]*MessageEntity, error) {
	query := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet
		WHERE contact_key = ? AND received_time > ?
		ORDER BY received_time ASC
		LIMIT ?
	`
	return d.queryMessages(query, contactKey, after, limit)
}

// GetByContactBefore retrieves messages for a contact before a timestamp (for pagination)
func (d *MessageDAO) GetByContactBefore(contactKey string, before int64, limit int) ([]*MessageEntity, error) {
	query := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet
		WHERE contact_key = ? AND received_time < ?
		ORDER BY received_time DESC
		LIMIT ?
	`
	return d.queryMessages(query, contactKey, before, limit)
}

// GetUnread retrieves unread messages for a contact
func (d *MessageDAO) GetUnread(contactKey string) ([]*MessageEntity, error) {
	query := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet
		WHERE contact_key = ? AND read = 0
		ORDER BY received_time ASC
	`
	return d.queryMessages(query, contactKey)
}

// GetAllUnread retrieves all unread messages
func (d *MessageDAO) GetAllUnread() ([]*MessageEntity, error) {
	query := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet
		WHERE read = 0
		ORDER BY received_time DESC
	`
	return d.queryMessages(query)
}

// GetRecent retrieves recent messages across all contacts
func (d *MessageDAO) GetRecent(limit int) ([]*MessageEntity, error) {
	query := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet
		ORDER BY received_time DESC
		LIMIT ?
	`
	return d.queryMessages(query, limit)
}

// GetByPortNum retrieves messages by port number
func (d *MessageDAO) GetByPortNum(portNum int32, limit int) ([]*MessageEntity, error) {
	query := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet
		WHERE port_num = ?
		ORDER BY received_time DESC
		LIMIT ?
	`
	return d.queryMessages(query, portNum, limit)
}

// Search searches messages by content
func (d *MessageDAO) Search(query string, limit int) ([]*MessageEntity, error) {
	searchPattern := "%" + query + "%"
	sqlQuery := `
		SELECT uuid, myNodeNum, port_num, contact_key, received_time, read,
			data, packet_id, routing_error, snr, rssi, hopsAway, filtered
		FROM packet
		WHERE data LIKE ?
		ORDER BY received_time DESC
		LIMIT ?
	`
	return d.queryMessages(sqlQuery, searchPattern, limit)
}

// MarkAsRead marks a message as read
func (d *MessageDAO) MarkAsRead(uuid int64) error {
	_, err := d.db.Exec("UPDATE packet SET read = 1 WHERE uuid = ?", uuid)
	return err
}

// MarkContactAsRead marks all messages from a contact as read
func (d *MessageDAO) MarkContactAsRead(contactKey string) error {
	_, err := d.db.Exec("UPDATE packet SET read = 1 WHERE contact_key = ? AND read = 0", contactKey)
	return err
}

// MarkAllAsRead marks all messages as read
func (d *MessageDAO) MarkAllAsRead() error {
	_, err := d.db.Exec("UPDATE packet SET read = 1 WHERE read = 0")
	return err
}

// CountUnread returns the count of unread messages for a contact
func (d *MessageDAO) CountUnread(contactKey string) (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM packet WHERE contact_key = ? AND read = 0", contactKey).Scan(&count)
	return count, err
}

// CountAllUnread returns the total count of unread messages
func (d *MessageDAO) CountAllUnread() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM packet WHERE read = 0").Scan(&count)
	return count, err
}

// DeleteByContact deletes all messages for a contact
func (d *MessageDAO) DeleteByContact(contactKey string) (int64, error) {
	result, err := d.db.Exec("DELETE FROM packet WHERE contact_key = ?", contactKey)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteOlderThan removes messages older than the given time
func (d *MessageDAO) DeleteOlderThan(cutoff time.Time) (int64, error) {
	result, err := d.db.Exec("DELETE FROM packet WHERE received_time < ?", cutoff.Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetContacts returns unique contact keys with their latest message time
func (d *MessageDAO) GetContacts() ([]ContactSummary, error) {
	query := `
		SELECT contact_key, MAX(received_time) as last_time,
			SUM(CASE WHEN read = 0 THEN 1 ELSE 0 END) as unread_count
		FROM packet
		GROUP BY contact_key
		ORDER BY last_time DESC
	`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []ContactSummary
	for rows.Next() {
		var c ContactSummary
		if err := rows.Scan(&c.ContactKey, &c.LastMessageTime, &c.UnreadCount); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// ContactSummary represents a contact with message summary
type ContactSummary struct {
	ContactKey      string
	LastMessageTime int64
	UnreadCount     int
}

// Reaction operations

// InsertReaction creates a new reaction
func (d *MessageDAO) InsertReaction(r *ReactionEntity) error {
	query := `
		INSERT OR REPLACE INTO reactions (
			myNodeNum, reply_id, user_id, emoji, timestamp,
			snr, rssi, hopsAway, packet_id, status, routing_error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.Exec(query,
		r.MyNodeNum, r.ReplyId, r.UserId, r.Emoji, r.Timestamp,
		r.SNR, r.RSSI, r.HopsAway, r.PacketId, r.Status, r.RoutingError,
	)
	return err
}

// GetReactionsByReplyId retrieves all reactions for a message
func (d *MessageDAO) GetReactionsByReplyId(replyId uint32) ([]*ReactionEntity, error) {
	query := `
		SELECT myNodeNum, reply_id, user_id, emoji, timestamp,
			snr, rssi, hopsAway, packet_id, status, routing_error
		FROM reactions
		WHERE reply_id = ?
		ORDER BY timestamp ASC
	`
	rows, err := d.db.Query(query, replyId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []*ReactionEntity
	for rows.Next() {
		r := &ReactionEntity{}
		err := rows.Scan(
			&r.MyNodeNum, &r.ReplyId, &r.UserId, &r.Emoji, &r.Timestamp,
			&r.SNR, &r.RSSI, &r.HopsAway, &r.PacketId, &r.Status, &r.RoutingError,
		)
		if err != nil {
			return nil, err
		}
		reactions = append(reactions, r)
	}
	return reactions, rows.Err()
}

// DeleteReaction removes a reaction
func (d *MessageDAO) DeleteReaction(replyId uint32, userId, emoji string) error {
	_, err := d.db.Exec(
		"DELETE FROM reactions WHERE reply_id = ? AND user_id = ? AND emoji = ?",
		replyId, userId, emoji,
	)
	return err
}

// Helper functions

func (d *MessageDAO) queryMessages(query string, args ...interface{}) ([]*MessageEntity, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*MessageEntity
	for rows.Next() {
		msg, err := scanMessageRows(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

func scanMessage(row *sql.Row) (*MessageEntity, error) {
	msg := &MessageEntity{}
	var read, filtered int
	err := row.Scan(
		&msg.UUID, &msg.MyNodeNum, &msg.PortNum, &msg.ContactKey,
		&msg.ReceivedTime, &read, &msg.Data, &msg.PacketId,
		&msg.RoutingError, &msg.SNR, &msg.RSSI, &msg.HopsAway, &filtered,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	msg.Read = read == 1
	msg.Filtered = filtered == 1
	return msg, nil
}

func scanMessageRows(rows *sql.Rows) (*MessageEntity, error) {
	msg := &MessageEntity{}
	var read, filtered int
	err := rows.Scan(
		&msg.UUID, &msg.MyNodeNum, &msg.PortNum, &msg.ContactKey,
		&msg.ReceivedTime, &read, &msg.Data, &msg.PacketId,
		&msg.RoutingError, &msg.SNR, &msg.RSSI, &msg.HopsAway, &filtered,
	)
	if err != nil {
		return nil, err
	}
	msg.Read = read == 1
	msg.Filtered = filtered == 1
	return msg, nil
}
