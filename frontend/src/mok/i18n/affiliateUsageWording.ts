// Upstream affiliate page sentences that describe recharge rebates, reworded for usage
// rebates. They are merged over the upstream messages while usage mode is on (see
// setAffiliateUsageWording in ./index.ts). src/mok/__tests__/i18n.spec.ts checks that every
// key still exists upstream.
export default {
  zh: {
    affiliate: {
      stats: {
        rebateRateHint: '被邀请用户每天消耗余额后，你按这个比例获得返利（次日结算）'
      },
      tips: {
        line2: '被邀请用户使用余额消费后，你可获得消费额 {rate} 的返利，每日结算一次。'
      }
    }
  },
  en: {
    affiliate: {
      stats: {
        rebateRateHint: 'The share of the daily balance usage of your invitees you earn, settled the next day'
      },
      tips: {
        line2: 'When invitees spend their balance, you receive {rate} of it as rebate quota, settled daily.'
      }
    }
  }
} as Record<string, Record<string, any>>
