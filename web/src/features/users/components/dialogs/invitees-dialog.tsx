/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatNumber, formatQuota, formatTimestamp } from '@/lib/format'

import { getSelfInvitees, getUserInvitees } from '../../api'
import { USER_STATUSES } from '../../constants'
import type { InviteeListItem, InviteeListResult } from '../../types'

type InviteesDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  inviterId?: number
}

function formatRebateRate(rate: number): string {
  return Number.isInteger(rate) ? String(rate) : rate.toFixed(2)
}

export function InviteesDialog({
  open,
  onOpenChange,
  inviterId,
}: InviteesDialogProps) {
  const { t } = useTranslation()
  const isAdminView = inviterId != null && inviterId > 0
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [data, setData] = useState<InviteeListResult | null>(null)
  const pageSize = 10

  useEffect(() => {
    if (open) {
      setPage(1)
    }
  }, [open, inviterId])

  useEffect(() => {
    if (!open) {
      return
    }
    let cancelled = false
    const load = async () => {
      setLoading(true)
      try {
        const result = isAdminView
          ? await getUserInvitees(inviterId, page, pageSize)
          : await getSelfInvitees(page, pageSize)
        if (cancelled) {
          return
        }
        if (result.success && result.data) {
          setData(result.data)
        } else {
          toast.error(result.message || t('Failed to load invitees'))
        }
      } catch {
        if (!cancelled) {
          toast.error(t('Failed to load invitees'))
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [open, inviterId, isAdminView, page, t])

  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / pageSize))
  const inviter = data?.inviter
  const items = data?.items ?? []
  const rebateRate = inviter?.effective_aff_rebate_rate ?? 0

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Invitees')}
      description={
        inviter
          ? t('Users invited by {{username}}', {
              username: inviter.display_name || inviter.username,
            })
          : t('Users who registered with this invitation')
      }
      contentClassName='flex max-h-[calc(100dvh-2rem)] flex-col sm:max-w-5xl'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      {inviter ? (
        <div className='grid gap-2 rounded-lg border p-3 sm:grid-cols-4'>
          <SummaryItem
            label={t('Invitation Code')}
            value={inviter.aff_code || '-'}
          />
          <SummaryItem
            label={t('Rebate Rate')}
            value={`${formatRebateRate(rebateRate)}% (${
              inviter.rebate_rate_source === 'exclusive'
                ? t('Exclusive')
                : t('Global')
            })`}
          />
          <SummaryItem
            label={t('Pending')}
            value={formatQuota(inviter.aff_quota)}
          />
          <SummaryItem
            label={t('Total Earned')}
            value={formatQuota(inviter.aff_history_quota)}
          />
        </div>
      ) : null}

      <div className='max-h-[min(54vh,520px)] overflow-auto rounded-lg border'>
        {loading && !data ? (
          <div className='space-y-2 p-4'>
            {Array.from({ length: 4 }).map((_, index) => (
              <Skeleton key={index} className='h-10 w-full' />
            ))}
          </div>
        ) : items.length === 0 ? (
          <div className='text-muted-foreground flex min-h-40 flex-col items-center justify-center py-10 text-center'>
            <p className='text-sm font-medium'>{t('No invitees yet')}</p>
            <p className='mt-1 text-xs'>
              {t('Users who register with this invitation will appear here')}
            </p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Username')}</TableHead>
                <TableHead>{t('Invited At')}</TableHead>
                <TableHead>{t('Last Login')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead className='text-right'>{t('Recharges')}</TableHead>
                <TableHead>{t('Last Top-up')}</TableHead>
                <TableHead className='text-right'>{t('Paid Amount')}</TableHead>
                {isAdminView ? (
                  <TableHead className='text-right'>{t('Quota')}</TableHead>
                ) : null}
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item) => (
                <InviteeRow
                  key={item.id}
                  item={item}
                  isAdminView={isAdminView}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <div className='flex items-center justify-between gap-2'>
        <p className='text-muted-foreground text-xs'>
          {t('{{count}} invitees', { count: data?.total ?? 0 })}
        </p>
        <div className='flex items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={page <= 1 || loading}
            onClick={() => setPage((current) => Math.max(1, current - 1))}
          >
            <ChevronLeft className='h-4 w-4' />
          </Button>
          <span className='text-muted-foreground text-xs tabular-nums'>
            {page} / {totalPages}
          </span>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={page >= totalPages || loading}
            onClick={() => setPage((current) => current + 1)}
          >
            <ChevronRight className='h-4 w-4' />
          </Button>
        </div>
      </div>
    </Dialog>
  )
}

function SummaryItem({ label, value }: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground text-[11px] tracking-wide uppercase'>
        {label}
      </div>
      <div className='truncate text-sm font-medium'>{value}</div>
    </div>
  )
}

function InviteeRow({
  item,
  isAdminView,
}: {
  item: InviteeListItem
  isAdminView: boolean
}) {
  const { t } = useTranslation()
  const status = USER_STATUSES[item.status as keyof typeof USER_STATUSES]
  return (
    <TableRow>
      <TableCell>
        <div className='min-w-0'>
          <div className='truncate font-medium'>
            {item.display_name || item.username}
          </div>
          <div className='text-muted-foreground truncate text-xs'>
            @{item.username} · #{item.id}
          </div>
        </div>
      </TableCell>
      <TableCell className='text-muted-foreground text-sm whitespace-nowrap'>
        {formatTimestamp(item.created_at)}
      </TableCell>
      <TableCell className='text-muted-foreground text-sm whitespace-nowrap'>
        {formatTimestamp(item.last_login_at)}
      </TableCell>
      <TableCell>
        {status ? (
          <StatusBadge
            label={t(status.labelKey)}
            variant={status.variant}
            copyable={false}
          />
        ) : (
          '-'
        )}
      </TableCell>
      <TableCell className='text-right tabular-nums'>
        {item.recharge_count}
      </TableCell>
      <TableCell className='text-muted-foreground text-sm whitespace-nowrap'>
        {item.last_recharged_at
          ? formatTimestamp(item.last_recharged_at)
          : '-'}
      </TableCell>
      <TableCell className='text-right tabular-nums'>
        {item.recharge_count > 0 ? formatNumber(item.total_pay_money) : '-'}
      </TableCell>
      {isAdminView ? (
        <TableCell className='text-right tabular-nums'>
          {item.quota == null ? '-' : formatQuota(item.quota)}
        </TableCell>
      ) : null}
    </TableRow>
  )
}
