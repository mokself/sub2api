// Upstream sentences the fork rewords for good, merged over the upstream messages whenever a
// locale is loaded (see mergeMokMessages in ./index.ts). src/mok/__tests__/i18n.spec.ts checks
// that every key still exists upstream with the same placeholders.
//
// The version badge: fork images report the "source" build type, so the badge offers the
// manual update path instead of upstream's online update (backend/internal/service/
// update_service_mok.go). These are the hints it shows there.
//
// The system settings tab that upstream calls "Email" also holds the fork's SMS card
// (src/mok/sms/SmsSettings.vue), so it is named after both.
export default {
  zh: {
    admin: {
      settings: {
        tabs: {
          email: '邮件与短信'
        }
      }
    },
    version: {
      sourceModeHint: 'Docker 部署：在服务器上运行 docker compose pull && docker compose up -d 更新',
      rollbackSourceHint: '不支持在线回退：把 docker-compose.yml 里的镜像标签改成旧版本（如 0.2.23-mok.1），再运行 docker compose up -d。回退前先看发版说明里有没有数据库迁移'
    }
  },
  en: {
    admin: {
      settings: {
        tabs: {
          email: 'Email & SMS'
        }
      }
    },
    version: {
      sourceModeHint: 'Docker deployment: run docker compose pull && docker compose up -d on the server to update',
      rollbackSourceHint: 'No online rollback: set the image tag in docker-compose.yml to an older version (such as 0.2.23-mok.1) and run docker compose up -d. Check the release notes for database migrations first'
    }
  }
} as Record<string, Record<string, any>>
