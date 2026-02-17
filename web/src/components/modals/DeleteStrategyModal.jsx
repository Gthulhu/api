import React, { useState, useEffect } from 'react';
import { useApp } from '../../context/AppContext';
import { Trash2, Target } from 'lucide-react';

export default function DeleteStrategyModal() {
  const { makeAuthenticatedRequest, showToast } = useApp();
  const [isOpen, setIsOpen] = useState(false);
  const [strategyId, setStrategyId] = useState('');

  useEffect(() => {
    const handleOpen = () => {
      setIsOpen(true);
    };
    
    window.addEventListener('openDeleteStrategyModal', handleOpen);
    return () => window.removeEventListener('openDeleteStrategyModal', handleOpen);
  }, []);

  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape' && isOpen) {
        handleClose();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen]);

  const handleClose = () => {
    setIsOpen(false);
    setStrategyId('');
  };

  const handleOverlayClick = (e) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  };

  const handleDelete = async () => {
    if (!strategyId.trim()) {
      showToast('error', 'Please enter a strategy ID');
      return;
    }
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/strategies', {
        method: 'DELETE',
        body: JSON.stringify({ strategyId: strategyId.trim() })
      });
      
      const data = await response.json();
      
      if (data.success) {
        showToast('success', 'Strategy deleted successfully');
        handleClose();
        window.dispatchEvent(new CustomEvent('refreshStrategies'));
        window.dispatchEvent(new CustomEvent('refreshIntents'));
      } else {
        showToast('error', data.error || data.message || 'Failed to delete strategy');
      }
    } catch (error) {
      showToast('error', 'Error: ' + error.message);
    }
  };

  if (!isOpen) return null;

  return (
    <div className={`modal-overlay ${isOpen ? 'active' : ''}`} onClick={handleOverlayClick}>
      <div className="modal-container">
        <div className="modal-header">
          <h2>
            <span className="modal-icon"><Trash2 size={18} /></span>
            Delete Schedule Strategy
          </h2>
          <button className="modal-close" onClick={handleClose}>×</button>
        </div>
        <div className="modal-body">
          <div className="input-group">
            <label htmlFor="deleteStrategyId">
              <span className="label-icon"><Target size={14} /></span>
              Strategy ID
            </label>
            <input 
              type="text" 
              id="deleteStrategyId" 
              placeholder="strategy-id"
              value={strategyId}
              onChange={(e) => setStrategyId(e.target.value)}
            />
            <div className="input-glow"></div>
          </div>
          <div className="form-actions">
            <button type="button" className="danger-btn" onClick={handleDelete}>
              <span className="btn-text">Delete Strategy</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
