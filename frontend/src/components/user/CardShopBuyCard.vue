<template>
  <div class="card p-6">
    <div class="flex items-start gap-4">
      <div
        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-amber-100 text-sm font-bold text-amber-700 dark:bg-amber-900/30 dark:text-amber-200"
      >
        1
      </div>
      <div class="min-w-0 flex-1">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('payment.cardShop.stepBuy') }}
        </h2>
        <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">
          {{ t('payment.cardShop.stepBuyDesc') }}
        </p>
      </div>
    </div>

    <SoftNoticeBanner
      v-if="notice.trim()"
      class="mt-5"
      linkify
      :title="noticeTitle || t('payment.pageNoticeTitle')"
      :text="notice.trim()"
    />

    <a
      v-if="shopUrl"
      data-test="card-shop-open"
      class="btn btn-primary mt-5 w-full py-3 text-base font-medium"
      :href="shopUrl"
      target="_blank"
      rel="noopener noreferrer"
    >
      {{ t('payment.cardShop.openShop') }}
      <Icon name="externalLink" size="sm" />
    </a>
    <p v-if="shopHost" class="mt-2 break-all text-center text-xs text-gray-400 dark:text-gray-500">
      {{ t('payment.cardShop.openShopHint', { host: shopHost }) }}
    </p>
    <p
      v-else
      class="mt-4 rounded-lg border border-dashed border-gray-200 px-4 py-3 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
    >
      {{ t('payment.cardShop.missingShop') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import SoftNoticeBanner from '@/components/common/SoftNoticeBanner.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractFirstHttpUrl } from '@/utils/httpTextLinks'

const props = withDefaults(defineProps<{
  notice?: string
  noticeTitle?: string
}>(), {
  notice: '',
  noticeTitle: '',
})

const { t } = useI18n()

const shopUrl = computed(() => extractFirstHttpUrl(props.notice || ''))
const shopHost = computed(() => {
  if (!shopUrl.value) return ''
  try {
    return new URL(shopUrl.value).host
  } catch {
    return ''
  }
})
</script>
