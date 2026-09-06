package debug

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"Eve/internal/bot/helpers"
	"Eve/internal/bot/ui"
	"Eve/internal/logger"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

const CommandName = "debug"

const commandTimeout = 5 * time.Second

const (
	msgOwnerOnly = "Cette commande est réservée au propriétaire du bot."
	msgGuildOnly = "Cette commande doit être utilisée dans un serveur."
	msgDBError   = "Erreur lors de l'accès à la base de données."
	msgRoleError = "Impossible de récupérer ou de créer le rôle « " + RoleName + " »."

	msgMissingPerm = "Je n'ai pas la permission **Administrateur**. " +
		"Accordez-la moi et placez mon rôle au-dessus du rôle « " + RoleName + " » pour que je puisse vous l'attribuer."

	msgEnabledFmt      = "Vous êtes maintenant en mode debug sur le serveur %s"
	msgDisabledFmt     = "Vous n'êtes plus en mode debug sur le serveur %s"
	msgEnabledFallback = "Vous êtes maintenant en mode debug sur ce serveur"
	msgDisabledFallbck = "Vous n'êtes plus en mode debug sur ce serveur"

	msgAddFailed    = "Impossible de vous attribuer le rôle « " + RoleName + " »."
	msgRemoveFailed = "Impossible de vous retirer le rôle « " + RoleName + " »."
)

var command = discord.SlashCommandCreate{
	Name:        CommandName,
	Description: "Activer ou désactiver le mode debug pour vous-même",
	Contexts:    []discord.InteractionContextType{discord.InteractionContextTypeGuild},
}

func Commands() []discord.ApplicationCommandCreate {
	if !helpers.OwnerConfigured() {
		return nil
	}
	return []discord.ApplicationCommandCreate{command}
}

func HandleCommand(e *events.ApplicationCommandInteractionCreate) {
	if !helpers.IsOwner(e.User().ID) {
		logger.Warn("Debug: refused non-owner caller",
			"user", e.User().ID.String(),
			"guild", guildIDString(e.GuildID()),
		)
		helpers.RespondEphemeralCard(e, ui.Error(msgOwnerOnly))
		return
	}

	guildID := e.GuildID()
	member := e.Member()
	if guildID == nil || member == nil {
		helpers.RespondEphemeralCard(e, ui.Error(msgGuildOnly))
		return
	}

	if perms := e.AppPermissions(); perms != nil && perms.Missing(discord.PermissionAdministrator) {
		logger.Debug("Debug: missing Administrator", "guild", guildID.String())
		helpers.RespondEphemeralCard(e, ui.Error(msgMissingPerm))
		return
	}

	if err := e.DeferCreateMessage(true); err != nil {
		logger.Error("Debug: deferring response failed", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	role, err := ensureRole(ctx, e, *guildID)
	if err != nil {
		switch {
		case errors.Is(err, errMissingPermissions):
			logger.Warn("Debug: role management refused by Discord", "guild", guildID.String(), "error", err)
			editDeferred(e, ui.Error(msgMissingPerm))
		case errors.Is(err, errStorage):
			logger.Error("Debug: guild config access failed", "guild", guildID.String(), "error", err)
			editDeferred(e, ui.Error(msgDBError))
		default:
			logger.Error("Debug: resolving debug role failed", "guild", guildID.String(), "error", err)
			editDeferred(e, ui.Error(msgRoleError))
		}
		return
	}

	if slices.Contains(member.RoleIDs, role.ID) {
		removeRole(ctx, e, *guildID, role.ID)
		return
	}
	addRole(ctx, e, *guildID, role.ID)
}

func addRole(ctx context.Context, e *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, roleID snowflake.ID) {
	userID := e.User().ID
	if err := e.Client().Rest.AddMemberRole(guildID, userID, roleID, rest.WithCtx(ctx)); err != nil {
		logger.Error("Debug: adding role failed", "guild", guildID.String(), "user", userID.String(), "error", err)
		editDeferred(e, ui.Error(toggleErrorMessage(err, msgAddFailed)))
		return
	}
	logger.Warn("Debug: administrator role granted to owner", "guild", guildID.String(), "user", userID.String(), "role", roleID.String())
	editDeferred(e, ui.Success(toggleMessage(ctx, e, guildID, msgEnabledFmt, msgEnabledFallback)))
}

func removeRole(ctx context.Context, e *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, roleID snowflake.ID) {
	userID := e.User().ID
	if err := e.Client().Rest.RemoveMemberRole(guildID, userID, roleID, rest.WithCtx(ctx)); err != nil {
		logger.Error("Debug: removing role failed", "guild", guildID.String(), "user", userID.String(), "error", err)
		editDeferred(e, ui.Error(toggleErrorMessage(err, msgRemoveFailed)))
		return
	}
	logger.Warn("Debug: administrator role revoked from owner", "guild", guildID.String(), "user", userID.String(), "role", roleID.String())
	editDeferred(e, ui.Success(toggleMessage(ctx, e, guildID, msgDisabledFmt, msgDisabledFallbck)))
}

func guildIDString(guildID *snowflake.ID) string {
	if guildID == nil {
		return ""
	}
	return guildID.String()
}

func toggleMessage(ctx context.Context, e *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, format string, fallback string) string {
	name, ok := guildName(ctx, e, guildID)
	if !ok {
		return fallback
	}
	return fmt.Sprintf(format, name)
}

func toggleErrorMessage(err error, fallback string) string {
	if isMissingPermissions(err) {
		return msgMissingPerm
	}
	return fallback
}

func editDeferred(e *events.ApplicationCommandInteractionCreate, card *ui.Card) {
	helpers.EditResponseCard(e.Client(), e.ApplicationID(), e.Token(), card)
}
