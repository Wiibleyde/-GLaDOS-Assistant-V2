import { prisma } from "./database";
import axios from "axios";
import fs from "fs";

class ICalendarManager {
    public guildId: string
    private calendarUrl: string
    private calendarData: string

    constructor(guildId: string, calendarUrl: string) {
        this.guildId = guildId
        this.calendarUrl = calendarUrl
        this.calendarData = ""
    }

    public async initCalendarUrl(): Promise<void> {
        this.calendarUrl = await prisma.guildData.findFirst({
            where: {
                guildId: this.guildId
            },
            select: {
                calendarUrl: true
            }
        }).then(data => data?.calendarUrl || "")
    }

    public async downloadCalendar(): Promise<void> {
        const response = await axios.get(this.calendarUrl)
        const file = fs.createWriteStream(`assets/cache/${this.guildId}.ics`)
        file.write(response.data)
        file.close()
        this.calendarData = await this.getCalendar()
    }

    public async getCalendar(): Promise<string> {
        if(!fs.existsSync(`assets/cache/${this.guildId}.ics`)) {
            await this.downloadCalendar()
        }
        return fs.readFileSync(`assets/cache/${this.guildId}.ics`).toString()
    }
}