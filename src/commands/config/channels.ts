import { CommandInteraction, EmbedBuilder, SlashCommandBuilder, PermissionFlagsBits } from "discord.js"
import { prisma } from "@/utils/database"
import { errorEmbed, successEmbed } from "@/utils/embeds"

export const data = new SlashCommandBuilder()
    .setName("channels")
    .setDescription("Configurer les salons")
    .addStringOption(option =>
        option
            .setName("action")
            .setDescription("L'action à effectuer")
            .addChoices({
                name: "Voir la configuration",
                value: "view"
            }, {
                name: "Modifier la configuration",
                value: "edit"
            })
            .setRequired(true)
    )
    .addStringOption(option =>
        option
            .setName("key")
            .setDescription("Clé de configuration")
            .setRequired(false)
            .addChoices({
                name: "Salon des anniversaires",
                value: "birthdayChannel"
            }, {
                name: "Salon des citations",
                value: "quoteChannel"
            })
    )
    .addChannelOption(option =>
        option
            .setName("channel")
            .setDescription("Salon")
            .setRequired(false)
    )
    .setDMPermission(false)
    .setDefaultMemberPermissions(PermissionFlagsBits.ManageChannels)

export async function execute(interaction: CommandInteraction) {
    const firstResponse = await interaction.deferReply({ ephemeral: true, fetchReply: true });
    switch (interaction.options.get("action")?.value) {
        case "view":
            const serverConfig = await prisma.config.findMany({
                where: {
                    guildId: parseInt(interaction.guildId?.toString() as string)
                }
            })
            if (serverConfig.length === 0) {
                await interaction.editReply({ embeds: [errorEmbed(interaction, new Error("Aucune configuration trouvée."))] })
                return
            }
            const embed = new EmbedBuilder()
                .setTitle("Configuration")
                .setDescription(serverConfig.map(config => `${config.key}: <#${config.value}>`).join("\n"))
                .setColor(0x00FF00)
                .setTimestamp()
                .setFooter({ text: `GLaDOS Assistant - Pour vous servir.`, iconURL: interaction.client.user.displayAvatarURL() })

            await interaction.editReply({ embeds: [embed] })
            break
        case "edit":
            const key = interaction.options.get("key")?.value as string
            const channel = interaction.options.get("channel")?.value as string
            if (!key || !channel) {
                await interaction.editReply({ embeds: [errorEmbed(interaction, new Error("Veuillez fournir une clé et un salon."))] })
                return
            }
            const existingConfig = await prisma.config.findFirst({
                where: {
                    guildId: parseInt(interaction.guildId?.toString() as string),
                    key
                }
            })
            if (existingConfig) {
                await prisma.config.update({
                    where: {
                        id: existingConfig.id
                    },
                    data: {
                        value: channel
                    }
                })
            } else {
                await prisma.config.create({
                    data: {
                        guildId: parseInt(interaction.guildId?.toString() as string),
                        key,
                        value: channel
                    }
                })
            }
            await interaction.editReply({ embeds: [successEmbed(interaction, "Configuration mise à jour.")] })
            break
    }
}