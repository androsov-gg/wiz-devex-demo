'use client';

import React from 'react';
import { Home, Layers, ShieldCheck, HelpCircle, Settings } from 'lucide-react';

interface SidebarNavProps {
  currentTab: 'home' | 'catalog';
  onTabChange: (tab: 'home' | 'catalog') => void;
}

export function SidebarNav({ currentTab, onTabChange }: SidebarNavProps) {
  const navItems = [
    { id: 'home' as const, label: 'Home', icon: Home, desc: 'Active Demos' },
    { id: 'catalog' as const, label: 'Catalog', icon: Layers, desc: 'Demo Catalog' },
  ];

  return (
    <aside className="w-20 bg-white border-r border-slate-200 flex flex-col items-center py-4 justify-between h-screen fixed left-0 top-0 z-30 select-none">
      <div className="flex flex-col items-center w-full gap-6">
        {/* Brand Logo */}
        <div
          onClick={() => onTabChange('home')}
          className="flex flex-col items-center justify-center cursor-pointer group"
        >
          <div className="w-10 h-10 rounded-xl bg-blue-50 border border-blue-200 flex items-center justify-center text-blue-600 shadow-xs group-hover:scale-105 transition-transform">
            <ShieldCheck className="w-6 h-6 text-blue-600" />
          </div>
          <span className="text-[10px] font-bold text-blue-600 mt-1 tracking-tight">Demo Hub</span>
        </div>

        {/* Navigation Items */}
        <nav className="flex flex-col items-center w-full gap-3 px-2">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = currentTab === item.id;
            return (
              <button
                key={item.id}
                onClick={() => onTabChange(item.id)}
                className={`w-14 h-14 rounded-xl flex flex-col items-center justify-center gap-1 transition-all ${
                  isActive
                    ? 'bg-blue-50 text-blue-600 border border-blue-200/80 shadow-xs font-semibold'
                    : 'text-slate-500 hover:text-slate-900 hover:bg-slate-100/80'
                }`}
                title={item.desc}
              >
                <Icon className={`w-5 h-5 ${isActive ? 'text-blue-600' : 'text-slate-500'}`} />
                <span className="text-[11px] leading-none">{item.label}</span>
              </button>
            );
          })}
        </nav>
      </div>

      {/* Bottom Nav Actions */}
      <div className="flex flex-col items-center gap-2 pb-2">
        <button
          className="w-10 h-10 rounded-lg flex items-center justify-center text-slate-400 hover:text-slate-700 hover:bg-slate-100 transition-colors"
          title="Help & Info"
        >
          <HelpCircle className="w-5 h-5" />
        </button>
      </div>
    </aside>
  );
}
