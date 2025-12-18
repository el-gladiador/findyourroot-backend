package handlers

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

// ReferentialIntegrityService handles cleanup of dangling references across collections
type ReferentialIntegrityService struct {
	client *firestore.Client
}

// NewReferentialIntegrityService creates a new integrity service
func NewReferentialIntegrityService(client *firestore.Client) *ReferentialIntegrityService {
	return &ReferentialIntegrityService{client: client}
}

// OnPersonDeleted cleans up all references when a person is deleted
// This should be called BEFORE the person is actually deleted
func (s *ReferentialIntegrityService) OnPersonDeleted(ctx context.Context, personID string) error {
	log.Printf("[RefIntegrity] Cleaning up references for deleted person: %s", personID)

	// 1. Clear person_id from users who were linked to this person
	if err := s.clearUserPersonLinks(ctx, personID); err != nil {
		log.Printf("[RefIntegrity] Warning: Failed to clear user links: %v", err)
	}

	// 2. Remove this person from any parent's children array
	if err := s.removeFromParentChildren(ctx, personID); err != nil {
		log.Printf("[RefIntegrity] Warning: Failed to remove from parent children: %v", err)
	}

	// 3. Handle orphaned children - they become root nodes (no parent)
	// Note: We don't delete children, just leave them as roots

	// 4. Reject/invalidate pending suggestions that reference this person
	if err := s.invalidateSuggestionsForPerson(ctx, personID); err != nil {
		log.Printf("[RefIntegrity] Warning: Failed to invalidate suggestions: %v", err)
	}

	// 5. Reject pending identity claims for this person
	if err := s.rejectIdentityClaimsForPerson(ctx, personID); err != nil {
		log.Printf("[RefIntegrity] Warning: Failed to reject identity claims: %v", err)
	}

	return nil
}

// OnUserDeleted cleans up all references when a user is deleted
func (s *ReferentialIntegrityService) OnUserDeleted(ctx context.Context, userID string) error {
	log.Printf("[RefIntegrity] Cleaning up references for deleted user: %s", userID)

	// 1. Clear linked_user_id from any person linked to this user
	if err := s.clearPersonUserLinks(ctx, userID); err != nil {
		log.Printf("[RefIntegrity] Warning: Failed to clear person links: %v", err)
	}

	// 2. Remove user from liked_by arrays
	if err := s.removeFromLikedBy(ctx, userID); err != nil {
		log.Printf("[RefIntegrity] Warning: Failed to remove from liked_by: %v", err)
	}

	// 3. Cancel pending permission requests from this user
	if err := s.cancelPermissionRequests(ctx, userID); err != nil {
		log.Printf("[RefIntegrity] Warning: Failed to cancel permission requests: %v", err)
	}

	// 4. Cancel pending identity claims from this user
	if err := s.cancelIdentityClaimsForUser(ctx, userID); err != nil {
		log.Printf("[RefIntegrity] Warning: Failed to cancel identity claims: %v", err)
	}

	// 5. Cancel pending suggestions from this user
	if err := s.cancelSuggestionsForUser(ctx, userID); err != nil {
		log.Printf("[RefIntegrity] Warning: Failed to cancel suggestions: %v", err)
	}

	// Note: We keep created_by and reviewed_by as historical records
	// They just point to a deleted user, which is fine for audit purposes

	return nil
}

// clearUserPersonLinks clears person_id from users linked to the deleted person
func (s *ReferentialIntegrityService) clearUserPersonLinks(ctx context.Context, personID string) error {
	iter := s.client.Collection("users").Where("person_id", "==", personID).Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		_, err = s.client.Collection("users").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "person_id", Value: ""},
			{Path: "tree_name", Value: ""},
			{Path: "updated_at", Value: time.Now()},
		})
		if err != nil {
			log.Printf("[RefIntegrity] Failed to clear user %s link: %v", doc.Ref.ID, err)
		} else {
			log.Printf("[RefIntegrity] Cleared person link for user %s", doc.Ref.ID)
		}
	}
	return nil
}

// removeFromParentChildren removes the person from any parent's children array
func (s *ReferentialIntegrityService) removeFromParentChildren(ctx context.Context, personID string) error {
	iter := s.client.Collection("people").Where("children", "array-contains", personID).Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		_, err = s.client.Collection("people").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "children", Value: firestore.ArrayRemove(personID)},
			{Path: "updated_at", Value: time.Now()},
		})
		if err != nil {
			log.Printf("[RefIntegrity] Failed to remove from parent %s children: %v", doc.Ref.ID, err)
		} else {
			log.Printf("[RefIntegrity] Removed person from parent %s children", doc.Ref.ID)
		}
	}
	return nil
}

// invalidateSuggestionsForPerson rejects pending suggestions targeting this person
func (s *ReferentialIntegrityService) invalidateSuggestionsForPerson(ctx context.Context, personID string) error {
	iter := s.client.Collection("suggestions").
		Where("target_person_id", "==", personID).
		Where("status", "==", "pending").
		Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		_, err = s.client.Collection("suggestions").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "status", Value: "rejected"},
			{Path: "review_notes", Value: "Auto-rejected: Target person was deleted"},
			{Path: "updated_at", Value: time.Now()},
		})
		if err != nil {
			log.Printf("[RefIntegrity] Failed to reject suggestion %s: %v", doc.Ref.ID, err)
		} else {
			log.Printf("[RefIntegrity] Auto-rejected suggestion %s (person deleted)", doc.Ref.ID)
		}
	}
	return nil
}

// rejectIdentityClaimsForPerson rejects pending claims for this person
func (s *ReferentialIntegrityService) rejectIdentityClaimsForPerson(ctx context.Context, personID string) error {
	iter := s.client.Collection("identity_claims").
		Where("person_id", "==", personID).
		Where("status", "==", "pending").
		Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		_, err = s.client.Collection("identity_claims").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "status", Value: "rejected"},
			{Path: "review_notes", Value: "Auto-rejected: Person was deleted from tree"},
			{Path: "updated_at", Value: time.Now()},
		})
		if err != nil {
			log.Printf("[RefIntegrity] Failed to reject identity claim %s: %v", doc.Ref.ID, err)
		} else {
			log.Printf("[RefIntegrity] Auto-rejected identity claim %s (person deleted)", doc.Ref.ID)
		}
	}
	return nil
}

// clearPersonUserLinks clears linked_user_id from people when user is deleted
func (s *ReferentialIntegrityService) clearPersonUserLinks(ctx context.Context, userID string) error {
	iter := s.client.Collection("people").Where("linked_user_id", "==", userID).Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		_, err = s.client.Collection("people").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "linked_user_id", Value: ""},
			{Path: "updated_at", Value: time.Now()},
		})
		if err != nil {
			log.Printf("[RefIntegrity] Failed to clear person %s user link: %v", doc.Ref.ID, err)
		} else {
			log.Printf("[RefIntegrity] Cleared user link for person %s", doc.Ref.ID)
		}
	}
	return nil
}

// removeFromLikedBy removes user from all liked_by arrays
func (s *ReferentialIntegrityService) removeFromLikedBy(ctx context.Context, userID string) error {
	iter := s.client.Collection("people").Where("liked_by", "array-contains", userID).Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		_, err = s.client.Collection("people").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "liked_by", Value: firestore.ArrayRemove(userID)},
			{Path: "likes_count", Value: firestore.Increment(-1)},
			{Path: "updated_at", Value: time.Now()},
		})
		if err != nil {
			log.Printf("[RefIntegrity] Failed to remove user from liked_by for person %s: %v", doc.Ref.ID, err)
		} else {
			log.Printf("[RefIntegrity] Removed user from liked_by for person %s", doc.Ref.ID)
		}
	}
	return nil
}

// cancelPermissionRequests cancels pending permission requests from deleted user
func (s *ReferentialIntegrityService) cancelPermissionRequests(ctx context.Context, userID string) error {
	iter := s.client.Collection("permission_requests").
		Where("user_id", "==", userID).
		Where("status", "==", "pending").
		Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		_, err = s.client.Collection("permission_requests").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "status", Value: "rejected"},
			{Path: "updated_at", Value: time.Now()},
		})
		if err != nil {
			log.Printf("[RefIntegrity] Failed to cancel permission request %s: %v", doc.Ref.ID, err)
		}
	}
	return nil
}

// cancelIdentityClaimsForUser cancels pending identity claims from deleted user
func (s *ReferentialIntegrityService) cancelIdentityClaimsForUser(ctx context.Context, userID string) error {
	iter := s.client.Collection("identity_claims").
		Where("user_id", "==", userID).
		Where("status", "==", "pending").
		Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		_, err = s.client.Collection("identity_claims").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "status", Value: "rejected"},
			{Path: "review_notes", Value: "Auto-rejected: User account deleted"},
			{Path: "updated_at", Value: time.Now()},
		})
		if err != nil {
			log.Printf("[RefIntegrity] Failed to cancel identity claim %s: %v", doc.Ref.ID, err)
		}
	}
	return nil
}

// cancelSuggestionsForUser cancels pending suggestions from deleted user
func (s *ReferentialIntegrityService) cancelSuggestionsForUser(ctx context.Context, userID string) error {
	iter := s.client.Collection("suggestions").
		Where("user_id", "==", userID).
		Where("status", "==", "pending").
		Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		_, err = s.client.Collection("suggestions").Doc(doc.Ref.ID).Update(ctx, []firestore.Update{
			{Path: "status", Value: "rejected"},
			{Path: "review_notes", Value: "Auto-rejected: User account deleted"},
			{Path: "updated_at", Value: time.Now()},
		})
		if err != nil {
			log.Printf("[RefIntegrity] Failed to cancel suggestion %s: %v", doc.Ref.ID, err)
		}
	}
	return nil
}

// ValidatePersonReferences checks if a person's references are valid and cleans up invalid ones
// Returns true if any cleanup was performed
func (s *ReferentialIntegrityService) ValidatePersonReferences(ctx context.Context, personID string) (bool, error) {
	doc, err := s.client.Collection("people").Doc(personID).Get(ctx)
	if err != nil {
		return false, err
	}

	var updates []firestore.Update
	changed := false

	// Check linked_user_id
	if linkedUserID, ok := doc.Data()["linked_user_id"].(string); ok && linkedUserID != "" {
		userDoc, err := s.client.Collection("users").Doc(linkedUserID).Get(ctx)
		if err != nil || !userDoc.Exists() {
			updates = append(updates, firestore.Update{Path: "linked_user_id", Value: ""})
			changed = true
			log.Printf("[RefIntegrity] Cleaning dangling linked_user_id %s from person %s", linkedUserID, personID)
		}
	}

	// Check children array
	if children, ok := doc.Data()["children"].([]interface{}); ok {
		var validChildren []string
		for _, childID := range children {
			if cid, ok := childID.(string); ok {
				childDoc, err := s.client.Collection("people").Doc(cid).Get(ctx)
				if err == nil && childDoc.Exists() {
					validChildren = append(validChildren, cid)
				} else {
					changed = true
					log.Printf("[RefIntegrity] Removing dangling child %s from person %s", cid, personID)
				}
			}
		}
		if changed {
			updates = append(updates, firestore.Update{Path: "children", Value: validChildren})
		}
	}

	// Check liked_by array
	if likedBy, ok := doc.Data()["liked_by"].([]interface{}); ok {
		var validLikedBy []string
		removedCount := 0
		for _, userID := range likedBy {
			if uid, ok := userID.(string); ok {
				userDoc, err := s.client.Collection("users").Doc(uid).Get(ctx)
				if err == nil && userDoc.Exists() {
					validLikedBy = append(validLikedBy, uid)
				} else {
					removedCount++
					log.Printf("[RefIntegrity] Removing dangling liked_by user %s from person %s", uid, personID)
				}
			}
		}
		if removedCount > 0 {
			changed = true
			updates = append(updates,
				firestore.Update{Path: "liked_by", Value: validLikedBy},
				firestore.Update{Path: "likes_count", Value: len(validLikedBy)},
			)
		}
	}

	if changed {
		updates = append(updates, firestore.Update{Path: "updated_at", Value: time.Now()})
		_, err = s.client.Collection("people").Doc(personID).Update(ctx, updates)
		if err != nil {
			return false, err
		}
	}

	return changed, nil
}

// ValidateUserReferences checks if a user's references are valid
func (s *ReferentialIntegrityService) ValidateUserReferences(ctx context.Context, userID string) (bool, error) {
	doc, err := s.client.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		return false, err
	}

	changed := false

	// Check person_id
	if personID, ok := doc.Data()["person_id"].(string); ok && personID != "" {
		personDoc, err := s.client.Collection("people").Doc(personID).Get(ctx)
		if err != nil || !personDoc.Exists() {
			_, err = s.client.Collection("users").Doc(userID).Update(ctx, []firestore.Update{
				{Path: "person_id", Value: ""},
				{Path: "tree_name", Value: ""},
				{Path: "updated_at", Value: time.Now()},
			})
			if err == nil {
				changed = true
				log.Printf("[RefIntegrity] Cleaned dangling person_id %s from user %s", personID, userID)
			}
		}
	}

	return changed, nil
}
