'use client';

import React from 'react';
import { Search, Plus, HelpCircle, Command } from 'lucide-react';
import { UserSession } from '@/lib/types';

interface TopHeaderProps {
  user: UserSession;
  currentTab: 'home' | 'catalog';
  searchQuery: string;
  onSearchChange: (q: string) => void;
  onOpenLaunchModal: () => void;
}

export function TopHeader({
  user,
  currentTab,
  searchQuery,
  onSearchChange,
  onOpenLaunchModal,
}: TopHeaderProps) {
  const isHome = currentTab === 'home';

  return (
    <header className="w-full bg-white border-b border-slate-200 px-8 py-5">
      {/* Top utility row: Search bar & User Avatar */}
      <div className="flex items-center justify-between gap-4 pb-4">
        <div className="text-xs font-medium text-slate-400 tracking-wider uppercase">
          Wiz DevEx Demo Platform &bull; Pre-Canned Hosted IDEs
        </div>

        <div className="flex items-center gap-4">
          {/* Global Search with Ctrl+K shortcut */}
          <div className="relative flex items-center">
            <Search className="w-4 h-4 text-slate-400 absolute left-3 pointer-events-none" />
            <input
              type="text"
              placeholder={isHome ? 'Search active demos...' : 'Search demo catalog...'}
              value={searchQuery}
              onChange={(e) => onSearchChange(e.target.value)}
              className="w-72 pl-9 pr-14 py-1.5 text-xs bg-slate-50 hover:bg-slate-100/80 focus:bg-white border border-slate-200 rounded-full focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all placeholder:text-slate-400"
            />
            <div className="absolute right-2.5 flex items-center gap-0.5 px-1.5 py-0.5 bg-white border border-slate-200 rounded text-[10px] text-slate-400 shadow-2xs font-mono pointer-events-none">
              <Command className="w-2.5 h-2.5" />
              <span>K</span>
            </div>
          </div>

          {/* User Avatar Circle with Initials */}
          <div
            className="w-8 h-8 rounded-full bg-blue-100 border border-blue-300 text-blue-700 flex items-center justify-center text-xs font-semibold shadow-2xs cursor-pointer"
            title={`Authenticated via IAP as ${user.email}`}
          >
            {user.initials}
          </div>
        </div>
      </div>

      {/* Main Page Title & Primary CTA */}
      <div className="flex items-center justify-between pt-1">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 tracking-tight">
            {isHome ? 'Active Demo Environments' : 'Available Demo Catalog'}
          </h1>
          <p className="text-xs text-slate-500 mt-0.5">
            {isHome
              ? 'Manage running Cloud Run demo instances, view live container logs, and connect directly to your IDE.'
              : 'Browse pre-configured Wiz sales demo environments loaded dynamically from Google Cloud Storage.'}
          </p>
        </div>

        {/* Royal Blue Primary Button */}
        <button
          onClick={onOpenLaunchModal}
          className="bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold px-4 py-2.5 rounded-lg shadow-sm hover:shadow transition-all flex items-center gap-2 active:scale-98"
        >
          <Plus className="w-4 h-4 stroke-[2.5]" />
          <span>Launch Demo</span>
        </button>
      </div>
    </header>
  );
}
